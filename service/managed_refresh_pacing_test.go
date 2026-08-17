package service

import (
	"encoding/base64"
	"encoding/json"
	"metapi/aggrsite/db"
	"metapi/aggrsite/platform"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func jwtIssuedFor(t *testing.T, issuedAt time.Time, lifetime time.Duration) string {
	t.Helper()
	payload, err := json.Marshal(map[string]interface{}{
		"iat": issuedAt.Unix(),
		"exp": issuedAt.Add(lifetime).Unix(),
	})
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}

func TestManagedRefreshLeadFollowsCredentialLifetime(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name     string
		lifetime time.Duration
		want     time.Duration
	}{
		// new-api access tokens: a quarter of 15 minutes is below the floor.
		{name: "short lived", lifetime: 15 * time.Minute, want: managedRefreshLead},
		{name: "medium", lifetime: 24 * time.Minute, want: 6 * time.Minute},
		{name: "long lived", lifetime: 24 * time.Hour, want: managedRefreshMaxLead},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := managedRefreshLeadFor(jwtIssuedFor(t, now, tc.lifetime)); got != tc.want {
				t.Fatalf("managedRefreshLeadFor = %s, want %s", got, tc.want)
			}
		})
	}

	if got := managedRefreshLeadFor("session=opaque-cookie"); got != managedRefreshLead {
		t.Fatalf("credentials without a lifetime must use the floor, got %s", got)
	}
	if managedRefreshLead <= managedRefreshInterval {
		t.Fatalf("the lead (%s) must exceed the scan interval (%s), otherwise a credential can expire between scans",
			managedRefreshLead, managedRefreshInterval)
	}
}

// Regression test for the cause of "access token 无效": with a fixed 10 minute lead a
// 15 minute new-api access token was due 5 minutes after it was issued, so every scan
// rotated it. That exhausted the upstream CriticalRateLimit budget on
// POST /api/user/auth/refresh (20 requests / 20 minutes per client IP), after which no
// account behind that IP could refresh at all and every token silently expired.
func TestFreshShortLivedTokenIsNotDueOnTheNextScan(t *testing.T) {
	now := time.Now()
	row := db.AccountWithSite{}

	row.AccessToken = jwtIssuedFor(t, now.Add(-6*time.Minute), 15*time.Minute)
	if isManagedRefreshDue(row, now) {
		t.Fatal("a 15 minute token used for 6 minutes must not be refreshed yet")
	}

	row.AccessToken = jwtIssuedFor(t, now.Add(-12*time.Minute), 15*time.Minute)
	if !isManagedRefreshDue(row, now) {
		t.Fatal("a token inside its last minutes must be refreshed")
	}
}

func TestManagedRefreshBackoffGrowsAndHonoursUpstreamHints(t *testing.T) {
	if got := managedRefreshBackoff(1, nil); got != managedRefreshBackoffBase {
		t.Fatalf("first failure backoff = %s, want %s", got, managedRefreshBackoffBase)
	}
	if got := managedRefreshBackoff(3, nil); got != 4*managedRefreshBackoffBase {
		t.Fatalf("third failure backoff = %s, want %s", got, 4*managedRefreshBackoffBase)
	}
	if got := managedRefreshBackoff(99, nil); got != managedRefreshBackoffMax {
		t.Fatalf("backoff must be capped at %s, got %s", managedRefreshBackoffMax, got)
	}
	if got := managedRefreshBackoff(1, &platform.RefreshResult{RetryAfter: 20 * time.Minute}); got != 20*time.Minute {
		t.Fatalf("upstream retry hint ignored: %s", got)
	}
	if got := managedRefreshBackoff(1, &platform.RefreshResult{CredentialDead: true}); got != managedRefreshDeadBackoff {
		t.Fatalf("a dead credential must not be probed every scan: %s", got)
	}
}

// A failing refresh must not be retried on every pass: the upstream route is rate
// limited per IP, so hammering it keeps every account on that site failing.
func TestEnsureManagedSessionBacksOffAfterFailure(t *testing.T) {
	setupManagedScanTestDB(t)

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"code":429,"message":"too many requests"}`))
	}))
	defer server.Close()

	dueAt := time.Now().Add(time.Minute).UnixMilli()
	row := newManagedAccount(t, server.URL, `{"sub2apiAuth":{"refreshToken":"refresh-old"},"managedAuth":{"tokenExpiresAt":`+
		jsonInt(dueAt)+`}}`)

	if _, _, didRefresh, err := EnsureManagedSession(row, nil); err == nil || didRefresh {
		t.Fatalf("expected the first attempt to fail, didRefresh=%v err=%v", didRefresh, err)
	}
	if calls == 0 {
		t.Fatal("the first attempt should have reached upstream")
	}

	callsAfterFirst := calls
	if _, _, didRefresh, err := EnsureManagedSession(row, nil); err != nil || didRefresh {
		t.Fatalf("a backing-off account must be skipped silently, didRefresh=%v err=%v", didRefresh, err)
	}
	if calls != callsAfterFirst {
		t.Fatalf("upstream was called again during the backoff window (%d -> %d)", callsAfterFirst, calls)
	}

	// A credential replaced from the outside (relogin, rebind) clears the backoff.
	state := getManagedRefreshState(row.ID)
	state.mu.Lock()
	state.syncCredential("token-pasted-by-operator")
	throttled := state.throttled(time.Now())
	state.mu.Unlock()
	if throttled {
		t.Fatal("a newly stored credential must be refreshable immediately")
	}
}

// Two refreshes in a row would rotate the upstream refresh token twice for nothing;
// the second caller simply held a stale row.
func TestForceRefreshIsCollapsedRightAfterASuccessfulRefresh(t *testing.T) {
	setupManagedScanTestDB(t)

	calls := 0
	server := sub2ApiRefreshServer(t, &calls)
	defer server.Close()

	dueAt := time.Now().Add(time.Minute).UnixMilli()
	row := newManagedAccount(t, server.URL, `{"sub2apiAuth":{"refreshToken":"refresh-old"},"managedAuth":{"tokenExpiresAt":`+
		jsonInt(dueAt)+`}}`)

	refreshed, _, didRefresh, err := EnsureManagedSession(row, nil)
	if err != nil || !didRefresh {
		t.Fatalf("expected a refresh, didRefresh=%v err=%v", didRefresh, err)
	}
	row.AccessToken = refreshed

	token, _, didRefresh, err := ForceRefreshManagedSession(row, nil)
	if err == nil || !isManagedRefreshDeferred(err) {
		t.Fatalf("ForceRefreshManagedSession should report the collapsed retry as deferred: %v", err)
	}
	if didRefresh {
		t.Fatal("a credential refreshed seconds ago must not be rotated again")
	}
	if token != refreshed {
		t.Fatalf("the current credential should be returned, got %q", token)
	}
	if calls != 1 {
		t.Fatalf("upstream refresh calls = %d, want 1", calls)
	}
}

func TestForceRefreshReturnsCredentialPersistedByAnotherWorker(t *testing.T) {
	setupManagedScanTestDB(t)

	calls := 0
	server := sub2ApiRefreshServer(t, &calls)
	defer server.Close()

	dueAt := time.Now().Add(time.Minute).UnixMilli()
	staleRow := newManagedAccount(t, server.URL, `{"sub2apiAuth":{"refreshToken":"refresh-old"},"managedAuth":{"tokenExpiresAt":`+
		jsonInt(dueAt)+`}}`)

	refreshed, _, didRefresh, err := EnsureManagedSession(staleRow, nil)
	if err != nil || !didRefresh {
		t.Fatalf("expected the first worker to refresh, didRefresh=%v err=%v", didRefresh, err)
	}

	token, _, didRefresh, err := ForceRefreshManagedSession(staleRow, nil)
	if err != nil {
		t.Fatalf("a stale caller should receive the already-persisted credential: %v", err)
	}
	if didRefresh {
		t.Fatal("the stale caller must not rotate the credential again")
	}
	if token != refreshed {
		t.Fatalf("returned token = %q, want %q", token, refreshed)
	}
	if calls != 1 {
		t.Fatalf("upstream refresh calls = %d, want 1", calls)
	}
}

func TestPermanentRefreshFailureSurvivesBackoffForAutoRelogin(t *testing.T) {
	setupManagedScanTestDB(t)

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":401,"reason":"REFRESH_TOKEN_INVALID","message":"invalid refresh token"}`))
	}))
	defer server.Close()

	dueAt := time.Now().Add(time.Minute).UnixMilli()
	row := newManagedAccount(t, server.URL, `{"sub2apiAuth":{"refreshToken":"refresh-old"},"managedAuth":{"tokenExpiresAt":`+
		jsonInt(dueAt)+`}}`)

	if _, _, _, err := EnsureManagedSession(row, nil); err == nil || !isManagedRefreshCredentialDead(err) {
		t.Fatalf("first permanent failure should be classified as dead: %v", err)
	}
	if _, _, _, err := ForceRefreshManagedSession(row, nil); err == nil || !isManagedRefreshCredentialDead(err) {
		t.Fatalf("forced recovery must retain the permanent classification during backoff: %v", err)
	}
	if calls != 1 {
		t.Fatalf("dead refresh credential was submitted again during backoff: calls=%d", calls)
	}
}

func TestManagedRefreshPersistenceFailureIsCredentialDead(t *testing.T) {
	setupManagedScanTestDB(t)

	calls := 0
	server := sub2ApiRefreshServer(t, &calls)
	defer server.Close()

	row := newManagedAccount(t, server.URL, `{"sub2apiAuth":{"refreshToken":"refresh-old"}}`)
	if _, err := db.Exec(`CREATE TRIGGER reject_managed_refresh BEFORE UPDATE ON accounts BEGIN SELECT RAISE(FAIL, 'write rejected'); END`); err != nil {
		t.Fatalf("create update trigger: %v", err)
	}

	if _, _, _, err := ForceRefreshManagedSession(row, nil); err == nil || !isManagedRefreshCredentialDead(err) {
		t.Fatalf("losing a rotated credential during persistence must require recovery: %v", err)
	}
	if calls != 1 {
		t.Fatalf("upstream refresh calls = %d, want 1", calls)
	}
}

func TestManagedRefreshStateFingerprintIncludesRefreshCredential(t *testing.T) {
	row := db.AccountWithSite{}
	row.AccessToken = "same-access"
	row.SitePlatform = "sub2api"
	row.SiteURL = "https://sub2.example"
	oldConfig := `{"sub2apiAuth":{"refreshToken":"refresh-old"}}`
	row.ExtraConfig = &oldConfig
	oldFingerprint := managedCredentialFingerprint(row)

	state := &managedRefreshState{}
	state.syncCredential(oldFingerprint)
	state.recordFailure(time.Now(), nil)
	if !state.throttled(time.Now()) {
		t.Fatal("the failed credential should be backed off")
	}

	newConfig := `{"sub2apiAuth":{"refreshToken":"refresh-new"}}`
	row.ExtraConfig = &newConfig
	state.syncCredential(managedCredentialFingerprint(row))
	if state.throttled(time.Now()) {
		t.Fatal("changing only the refresh credential must clear the old backoff")
	}
}

func TestManagedRefreshScopeTracksSystemProxyChanges(t *testing.T) {
	setupManagedScanTestDB(t)

	useSystemProxy := true
	row := db.AccountWithSite{}
	row.SitePlatform = "new-api-v1"
	row.SiteURL = "https://new-api.example"
	row.SiteUseSystemProxy = &useSystemProxy
	opt := requestOptionForAccount(row)

	if err := db.UpsertSetting("system_proxy_url", "http://127.0.0.1:7890"); err != nil {
		t.Fatalf("set first system proxy: %v", err)
	}
	firstScope := managedRefreshSiteScope(row, opt)
	firstFingerprint := managedCredentialFingerprint(row)

	if err := db.UpsertSetting("system_proxy_url", "http://127.0.0.1:7891"); err != nil {
		t.Fatalf("set second system proxy: %v", err)
	}
	if secondScope := managedRefreshSiteScope(row, opt); secondScope == firstScope {
		t.Fatal("changing the actual system proxy route must create a new site pacing scope")
	}
	if secondFingerprint := managedCredentialFingerprint(row); secondFingerprint == firstFingerprint {
		t.Fatal("changing the actual system proxy route must clear the account backoff fingerprint")
	}
}

func TestManagedRefreshPacesAccountsSharingASite(t *testing.T) {
	setupManagedScanTestDB(t)

	calls := 0
	server := sub2ApiRefreshServer(t, &calls)
	defer server.Close()

	siteID, err := db.CreateSite(db.CreateSiteInput{Name: "shared-sub2", URL: server.URL, Platform: "sub2api", Status: "active"})
	if err != nil {
		t.Fatalf("CreateSite failed: %v", err)
	}
	newAccount := func(username string) db.AccountWithSite {
		t.Helper()
		accountID, createErr := db.CreateAccount(db.CreateAccountInput{
			SiteID:         siteID,
			Username:       username,
			AccessToken:    "jwt-" + username,
			Status:         "active",
			CheckinEnabled: true,
		})
		if createErr != nil {
			t.Fatalf("CreateAccount(%s) failed: %v", username, createErr)
		}
		extraConfig := `{"sub2apiAuth":{"refreshToken":"refresh-` + username + `"},"managedAuth":{"tokenExpiresAt":` + jsonInt(time.Now().Add(time.Minute).UnixMilli()) + `}}`
		if updateErr := db.UpdateAccount(accountID, map[string]interface{}{"extra_config": extraConfig}); updateErr != nil {
			t.Fatalf("UpdateAccount(%s) failed: %v", username, updateErr)
		}
		row, getErr := db.GetAccountWithSite(accountID)
		if getErr != nil {
			t.Fatalf("GetAccountWithSite(%s) failed: %v", username, getErr)
		}
		return *row
	}

	first := newAccount("one")
	second := newAccount("two")
	if _, _, didRefresh, err := EnsureManagedSession(first, nil); err != nil || !didRefresh {
		t.Fatalf("first shared-site account should refresh, didRefresh=%v err=%v", didRefresh, err)
	}
	if _, _, didRefresh, err := EnsureManagedSession(second, nil); err == nil || didRefresh || !isManagedRefreshDeferred(err) {
		t.Fatalf("second account should be deferred by the shared site budget, didRefresh=%v err=%v", didRefresh, err)
	}
	if calls != 1 {
		t.Fatalf("shared-site refresh calls = %d, want 1", calls)
	}
}

func TestManagedRefreshSchedulerWakesForNextSharedSiteSlot(t *testing.T) {
	setupManagedScanTestDB(t)

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{"access_token":"jwt-refreshed","refresh_token":"refresh-next","expires_in":3600}}`))
	}))
	defer server.Close()

	dueAt := time.Now().Add(time.Minute).UnixMilli()
	extraConfig := `{"sub2apiAuth":{"refreshToken":"refresh-old"},"managedAuth":{"tokenExpiresAt":` + jsonInt(dueAt) + `}}`
	_ = newManagedAccount(t, server.URL, extraConfig)
	_ = newManagedAccount(t, server.URL, extraConfig)

	StartManagedRefreshScheduler()
	t.Cleanup(StopManagedRefreshScheduler)
	deadline := time.Now().Add(5 * time.Second)
	for calls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("deferred shared-site account was not refreshed in its 2-second slot: calls=%d", got)
	}
}
