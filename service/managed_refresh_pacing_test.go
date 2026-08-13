package service

import (
	"encoding/base64"
	"encoding/json"
	"metapi/aggrsite/db"
	"metapi/aggrsite/platform"
	"net/http"
	"net/http/httptest"
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
	if err != nil {
		t.Fatalf("ForceRefreshManagedSession failed: %v", err)
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
