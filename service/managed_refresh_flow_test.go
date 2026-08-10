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

func sub2ApiRefreshServer(t *testing.T, calls *int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/refresh" {
			http.NotFound(w, r)
			return
		}
		*calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{"access_token":"jwt-refreshed","refresh_token":"refresh-next","expires_in":3600}}`))
	}))
}

func newManagedAccount(t *testing.T, siteURL, extraConfig string) db.AccountWithSite {
	t.Helper()

	siteID, err := db.CreateSite(db.CreateSiteInput{Name: "sub2api", URL: siteURL, Platform: "sub2api", Status: "active"})
	if err != nil {
		t.Fatalf("CreateSite failed: %v", err)
	}
	accountID, err := db.CreateAccount(db.CreateAccountInput{
		SiteID:         siteID,
		Username:       "managed",
		AccessToken:    "jwt-old",
		CheckinEnabled: true,
		Status:         "active",
	})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}
	if err := db.UpdateAccount(accountID, map[string]interface{}{"extra_config": extraConfig}); err != nil {
		t.Fatalf("UpdateAccount failed: %v", err)
	}
	row, err := db.GetAccountWithSite(accountID)
	if err != nil {
		t.Fatalf("GetAccountWithSite failed: %v", err)
	}
	return *row
}

// A forced refresh is the recovery path for an expired credential, so it must run
// even when no expiry was ever recorded (tokenExpiresAt is optional in the UI).
func TestForceRefreshManagedSessionWorksWithoutRecordedExpiry(t *testing.T) {
	setupManagedScanTestDB(t)

	calls := 0
	server := sub2ApiRefreshServer(t, &calls)
	defer server.Close()

	row := newManagedAccount(t, server.URL, `{"sub2apiAuth":{"refreshToken":"refresh-old"}}`)

	accessToken, extraConfig, didRefresh, err := ForceRefreshManagedSession(row, nil)
	if err != nil {
		t.Fatalf("ForceRefreshManagedSession failed: %v", err)
	}
	if !didRefresh || accessToken != "jwt-refreshed" {
		t.Fatalf("expected a refresh, got token=%q didRefresh=%v", accessToken, didRefresh)
	}
	if calls != 1 {
		t.Fatalf("upstream refresh calls = %d, want 1", calls)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(extraConfig), &cfg); err != nil {
		t.Fatalf("returned extraConfig is not json: %v", err)
	}
	managed, _ := cfg["managedAuth"].(map[string]interface{})
	if managed == nil || int64(managed["tokenExpiresAt"].(float64)) <= time.Now().UnixMilli() {
		t.Fatalf("expiry not recorded for later passes: %#v", cfg)
	}

	stored, err := db.GetAccount(row.ID)
	if err != nil {
		t.Fatalf("GetAccount failed: %v", err)
	}
	if stored.AccessToken != "jwt-refreshed" {
		t.Fatalf("refreshed token not persisted: %q", stored.AccessToken)
	}
}

// Without force the refresher stays passive when nothing hints at an expiry:
// no recorded value and a credential that is not a JWT (opaque session string).
func TestEnsureManagedSessionSkipsWhenExpiryUndeterminable(t *testing.T) {
	setupManagedScanTestDB(t)

	calls := 0
	server := sub2ApiRefreshServer(t, &calls)
	defer server.Close()

	row := newManagedAccount(t, server.URL, `{"sub2apiAuth":{"refreshToken":"refresh-old"}}`)

	accessToken, _, didRefresh, err := EnsureManagedSession(row, nil)
	if err != nil {
		t.Fatalf("EnsureManagedSession failed: %v", err)
	}
	if didRefresh || accessToken != "jwt-old" {
		t.Fatalf("expected no refresh, got token=%q didRefresh=%v", accessToken, didRefresh)
	}
	if calls != 0 {
		t.Fatalf("upstream refresh calls = %d, want 0", calls)
	}
}

func TestEnsureManagedSessionRefreshesTokenNearingExpiry(t *testing.T) {
	setupManagedScanTestDB(t)

	calls := 0
	server := sub2ApiRefreshServer(t, &calls)
	defer server.Close()

	dueAt := time.Now().Add(time.Minute).UnixMilli()
	row := newManagedAccount(t, server.URL, `{"sub2apiAuth":{"refreshToken":"refresh-old"},"managedAuth":{"tokenExpiresAt":`+
		jsonInt(dueAt)+`}}`)

	accessToken, _, didRefresh, err := EnsureManagedSession(row, nil)
	if err != nil {
		t.Fatalf("EnsureManagedSession failed: %v", err)
	}
	if !didRefresh || accessToken != "jwt-refreshed" {
		t.Fatalf("expected a refresh, got token=%q didRefresh=%v", accessToken, didRefresh)
	}
	if calls != 1 {
		t.Fatalf("upstream refresh calls = %d, want 1", calls)
	}
}

func TestResolveCheckinCredentialPrefersExplicitCredential(t *testing.T) {
	explicit := `{"checkin_credential":"cookie=explicit"}`
	row := db.AccountWithSite{}
	row.AccessToken = "token-fresh"
	row.ExtraConfig = &explicit
	if got := resolveCheckinCredential(row); got != "cookie=explicit" {
		t.Fatalf("resolveCheckinCredential = %q", got)
	}

	// No explicit credential: the (possibly just refreshed) access token is used.
	empty := `{"credentialMode":"session"}`
	row.ExtraConfig = &empty
	if got := resolveCheckinCredential(row); got != "token-fresh" {
		t.Fatalf("resolveCheckinCredential = %q", got)
	}

	row.ExtraConfig = nil
	if got := resolveCheckinCredential(row); got != "token-fresh" {
		t.Fatalf("resolveCheckinCredential = %q", got)
	}
}

func jsonInt(value int64) string {
	out, _ := json.Marshal(value)
	return string(out)
}

func testAccessTokenExpiringAt(t *testing.T, expiresAt time.Time) string {
	t.Helper()
	payload, err := json.Marshal(map[string]interface{}{"sub": "1891", "exp": expiresAt.Unix()})
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}

// An account imported without tokenExpiresAt must still be refreshed proactively:
// the JWT credential itself says when it dies.
func TestEnsureManagedSessionUsesJwtExpiryWhenNoneRecorded(t *testing.T) {
	setupManagedScanTestDB(t)

	calls := 0
	server := sub2ApiRefreshServer(t, &calls)
	defer server.Close()

	row := newManagedAccount(t, server.URL, `{"sub2apiAuth":{"refreshToken":"refresh-old"}}`)
	dueToken := testAccessTokenExpiringAt(t, time.Now().Add(time.Minute))
	if err := db.UpdateAccount(row.ID, map[string]interface{}{"access_token": dueToken}); err != nil {
		t.Fatalf("UpdateAccount failed: %v", err)
	}
	row.AccessToken = dueToken

	accessToken, _, didRefresh, err := EnsureManagedSession(row, nil)
	if err != nil {
		t.Fatalf("EnsureManagedSession failed: %v", err)
	}
	if !didRefresh || accessToken != "jwt-refreshed" {
		t.Fatalf("expected a refresh driven by the JWT exp claim, got token=%q didRefresh=%v", accessToken, didRefresh)
	}
	if calls != 1 {
		t.Fatalf("upstream refresh calls = %d, want 1", calls)
	}
}

func TestEnsureManagedSessionLeavesFreshJwtAlone(t *testing.T) {
	setupManagedScanTestDB(t)

	calls := 0
	server := sub2ApiRefreshServer(t, &calls)
	defer server.Close()

	row := newManagedAccount(t, server.URL, `{"sub2apiAuth":{"refreshToken":"refresh-old"}}`)
	freshToken := testAccessTokenExpiringAt(t, time.Now().Add(2*time.Hour))
	if err := db.UpdateAccount(row.ID, map[string]interface{}{"access_token": freshToken}); err != nil {
		t.Fatalf("UpdateAccount failed: %v", err)
	}
	row.AccessToken = freshToken

	_, _, didRefresh, err := EnsureManagedSession(row, nil)
	if err != nil {
		t.Fatalf("EnsureManagedSession failed: %v", err)
	}
	if didRefresh || calls != 0 {
		t.Fatalf("a token far from expiry must not be refreshed (didRefresh=%v calls=%d)", didRefresh, calls)
	}
}

func TestManagedTokenExpiresAtPrefersRecordedValue(t *testing.T) {
	recorded := time.Now().Add(3 * time.Hour).UnixMilli()
	extraConfig := `{"managedAuth":{"tokenExpiresAt":` + jsonInt(recorded) + `}}`

	row := db.AccountWithSite{}
	row.AccessToken = testAccessTokenExpiringAt(t, time.Now().Add(time.Minute))
	row.ExtraConfig = &extraConfig
	if got := managedTokenExpiresAt(row); got != recorded {
		t.Fatalf("managedTokenExpiresAt = %d, want the recorded value %d", got, recorded)
	}

	row.ExtraConfig = nil
	if got := managedTokenExpiresAt(row); got != platform.JwtExpiresAtMillis(row.AccessToken) || got <= 0 {
		t.Fatalf("managedTokenExpiresAt should fall back to the JWT exp, got %d", got)
	}
}
