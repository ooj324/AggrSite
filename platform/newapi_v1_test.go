package platform

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewApiV1AdapterIsRegisteredAndInheritsNewApiBehaviour(t *testing.T) {
	adapter := GetAdapter("new-api-v1")
	if adapter == nil {
		t.Fatal("new-api-v1 adapter is not registered")
	}
	if adapter.PlatformName() != "new-api-v1" {
		t.Fatalf("PlatformName = %q", adapter.PlatformName())
	}
	if GetAdapter("new-api").PlatformName() != "new-api" {
		t.Fatal("new-api-v1 registration must not shadow new-api")
	}
	if _, ok := adapter.(*NewApiV1Adapter); !ok {
		t.Fatalf("unexpected adapter type %T", adapter)
	}
}

func TestNewApiV1RefreshAuthRotatesCookieAndStoresCanonicalExpiry(t *testing.T) {
	expiresAtSeconds := time.Now().Add(time.Hour).Unix()
	var gotCookie, gotOrigin, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/user/auth/refresh" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		gotMethod = r.Method
		gotCookie = r.Header.Get("Cookie")
		gotOrigin = r.Header.Get("Origin")
		w.Header().Add("Set-Cookie", "new_api_refresh=rotated-refresh; Path=/; HttpOnly")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"access_token":"jwt-new","access_expires_at":` +
			jsonNumber(expiresAtSeconds) + `}}`))
	}))
	defer server.Close()

	adapter := &NewApiV1Adapter{NewApiAdapter{BaseAdapter: BaseAdapter{Name: "new-api-v1"}}}
	extraConfig := `{"credentialMode":"session","newApiV1Auth":{"refreshCookie":"old-refresh"},"sub2apiAuth":{"tokenExpiresAt":1}}`

	res, err := adapter.RefreshAuth(server.URL, "jwt-old", extraConfig, nil)
	if err != nil {
		t.Fatalf("RefreshAuth returned error: %v", err)
	}
	if res == nil || !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
	if res.AccessToken != "jwt-new" {
		t.Fatalf("AccessToken = %q", res.AccessToken)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q", gotMethod)
	}
	if !strings.Contains(gotCookie, "new_api_refresh=old-refresh") {
		t.Fatalf("refresh cookie not sent, got %q", gotCookie)
	}
	if gotOrigin != server.URL {
		t.Fatalf("Origin = %q, want %q (upstream guards the route with SessionCookieOriginGuard)", gotOrigin, server.URL)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(res.ExtraConfig), &cfg); err != nil {
		t.Fatalf("ExtraConfig is not valid json: %v", err)
	}
	authNode, _ := cfg["newApiV1Auth"].(map[string]interface{})
	if authNode == nil || authNode["refreshCookie"] != "rotated-refresh" {
		t.Fatalf("rotated refresh cookie not persisted: %#v", cfg["newApiV1Auth"])
	}
	managed, _ := cfg["managedAuth"].(map[string]interface{})
	if managed == nil {
		t.Fatalf("managedAuth missing: %#v", cfg)
	}
	if got, want := int64(managed["tokenExpiresAt"].(float64)), expiresAtSeconds*1000; got != want {
		t.Fatalf("tokenExpiresAt = %d, want %d (unix millis)", got, want)
	}
	if _, exists := cfg["sub2apiAuth"]; exists {
		t.Fatalf("legacy expiry copy must be dropped: %#v", cfg)
	}
	if cfg["credentialMode"] != "session" {
		t.Fatalf("unrelated config must be preserved: %#v", cfg)
	}
}

func TestNewApiV1RefreshAuthFallsBackToCookieJarCredential(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Set-Cookie", "new_api_refresh=rotated-from-jar; Path=/api/user/auth; HttpOnly")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"access_token":"jwt-new"}}`))
	}))
	defer server.Close()

	adapter := &NewApiV1Adapter{NewApiAdapter{BaseAdapter: BaseAdapter{Name: "new-api-v1"}}}
	res, err := adapter.RefreshAuth(server.URL, "session=abc; new_api_refresh=from-jar", "", nil)
	if err != nil {
		t.Fatalf("RefreshAuth returned error: %v", err)
	}
	if res == nil || !res.Success || res.AccessToken != "jwt-new" {
		t.Fatalf("expected success from cookie-jar refresh token, got %+v", res)
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(res.ExtraConfig), &cfg); err != nil {
		t.Fatalf("ExtraConfig is not valid json: %v", err)
	}
	managed, _ := cfg["managedAuth"].(map[string]interface{})
	if managed == nil || int64(managed["tokenExpiresAt"].(float64)) <= time.Now().UnixMilli() {
		t.Fatalf("expected a fallback expiry in the future: %#v", cfg)
	}
	authNode, _ := cfg["newApiV1Auth"].(map[string]interface{})
	if authNode == nil || authNode["refreshCookie"] != "rotated-from-jar" {
		t.Fatalf("rotated cookie-jar credential was not stored: %#v", cfg)
	}
}

func TestNewApiV1RefreshAuthRejectsSuccessWithoutRotatedCookie(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"access_token":"jwt-new"}}`))
	}))
	defer server.Close()

	adapter := &NewApiV1Adapter{NewApiAdapter{BaseAdapter: BaseAdapter{Name: "new-api-v1"}}}
	res, err := adapter.RefreshAuth(server.URL, "jwt-old", `{"newApiV1Auth":{"refreshCookie":"old"}}`, nil)
	if err != nil {
		t.Fatalf("RefreshAuth returned error: %v", err)
	}
	if res == nil || res.Success {
		t.Fatalf("a successful rotation without Set-Cookie must fail, got %+v", res)
	}
	// Missing Set-Cookie is NOT marked as CredentialDead: the old cookie may still
	// be within its upstream replay grace window, and the scheduler's exponential
	// backoff will retry. Marking it dead would force a re-login for a transient
	// proxy issue.
	if res.CredentialDead {
		t.Fatalf("missing Set-Cookie should NOT be marked as CredentialDead, got %+v", res)
	}
	if !strings.Contains(strings.ToLower(res.Message), "rotated refresh cookie") {
		t.Fatalf("unexpected message: %q", res.Message)
	}
}

// RefreshAuth must make exactly one HTTP request and never replay the cookie.
// If the Set-Cookie is missing, it should fail without retrying.
func TestNewApiV1RefreshAuthDoesNotReplayOnMissingCookie(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"access_token":"jwt-new"}}`))
	}))
	defer server.Close()

	adapter := &NewApiV1Adapter{NewApiAdapter{BaseAdapter: BaseAdapter{Name: "new-api-v1"}}}
	res, err := adapter.RefreshAuth(server.URL, "jwt-old", `{"newApiV1Auth":{"refreshCookie":"old"}}`, nil)
	if err != nil {
		t.Fatalf("RefreshAuth returned error: %v", err)
	}
	if res == nil || res.Success {
		t.Fatalf("missing Set-Cookie must fail, got %+v", res)
	}
	if calls != 1 {
		t.Fatalf("RefreshAuth must make exactly 1 HTTP request, got %d", calls)
	}
}

// AUTH_REFRESH_RACE is classified with a short RetryAfter so the scheduler
// retries quickly instead of replaying the cookie inline.
func TestNewApiV1RefreshAuthClassifiesRefreshRaceWithoutReplay(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"success":false,"code":"AUTH_REFRESH_RACE","message":"Conflict"}`))
	}))
	defer server.Close()

	adapter := &NewApiV1Adapter{NewApiAdapter{BaseAdapter: BaseAdapter{Name: "new-api-v1"}}}
	res, err := adapter.RefreshAuth(server.URL, "jwt-old", `{"newApiV1Auth":{"refreshCookie":"old"}}`, nil)
	if err != nil {
		t.Fatalf("RefreshAuth returned error: %v", err)
	}
	if res == nil || res.Success {
		t.Fatalf("AUTH_REFRESH_RACE must fail, got %+v", res)
	}
	if calls != 1 {
		t.Fatalf("RefreshAuth must not replay the cookie, got %d calls", calls)
	}
	if res.RetryAfter != newApiV1RefreshRaceBackoff {
		t.Fatalf("RetryAfter = %s, want %s", res.RetryAfter, newApiV1RefreshRaceBackoff)
	}
	if res.CredentialDead {
		t.Fatalf("AUTH_REFRESH_RACE is not a dead credential")
	}
}

// Transport errors (connection reset, timeout) are classified as failures
// without replaying the cookie — the outcome is uncertain and replay risks
// triggering upstream refresh-reuse detection.
func TestNewApiV1TransportErrorDoesNotReplay(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "hijacking unsupported", http.StatusInternalServerError)
			return
		}
		conn, _, err := hijacker.Hijack()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = conn.Close()
	}))
	defer server.Close()

	adapter := &NewApiV1Adapter{NewApiAdapter{BaseAdapter: BaseAdapter{Name: "new-api-v1"}}}
	res, err := adapter.RefreshAuth(server.URL, "jwt-old", `{"newApiV1Auth":{"refreshCookie":"old"}}`, nil)
	if err != nil {
		t.Fatalf("RefreshAuth returned error: %v", err)
	}
	if res == nil || res.Success {
		t.Fatalf("transport error must fail, got %+v", res)
	}
	if calls != 1 {
		t.Fatalf("RefreshAuth must not replay after transport error, got %d calls", calls)
	}
}

func TestNewApiV1RefreshAuthWithoutRefreshCredential(t *testing.T) {
	adapter := &NewApiV1Adapter{NewApiAdapter{BaseAdapter: BaseAdapter{Name: "new-api-v1"}}}
	res, err := adapter.RefreshAuth("http://127.0.0.1:1", "jwt-old", `{"credentialMode":"session"}`, nil)
	if err != nil {
		t.Fatalf("RefreshAuth returned error: %v", err)
	}
	if res == nil || res.Success {
		t.Fatalf("expected failure without refresh cookie, got %+v", res)
	}
	if !strings.Contains(res.Message, "refresh cookie") {
		t.Fatalf("unexpected message: %q", res.Message)
	}
}

func TestNewApiV1RefreshAuthReportsUpstreamFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":false,"message":"refresh token expired"}`))
	}))
	defer server.Close()

	adapter := &NewApiV1Adapter{NewApiAdapter{BaseAdapter: BaseAdapter{Name: "new-api-v1"}}}
	res, err := adapter.RefreshAuth(server.URL, "jwt-old", `{"newApiV1Auth":{"refreshCookie":"old"}}`, nil)
	if err != nil {
		t.Fatalf("RefreshAuth returned error: %v", err)
	}
	if res == nil || res.Success || res.Message != "refresh token expired" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

// The refresh route is guarded upstream by CriticalRateLimit (20 requests / 20
// minutes per client IP) and answers auth failures with a code rather than a
// meaningful message. Both have to be reported back, otherwise the scheduler keeps
// retrying and holds the limiter saturated for every account on the same IP.
func TestNewApiV1RefreshAuthClassifiesHTTPFailures(t *testing.T) {
	cases := []struct {
		name               string
		status             int
		body               string
		retryAfterHeader   string
		wantRetryAfter     time.Duration
		wantCredentialDead bool
	}{
		{
			name:           "rate limited",
			status:         http.StatusTooManyRequests,
			body:           `{"success":false,"message":"请求过于频繁"}`,
			wantRetryAfter: newApiV1RateLimitBackoff,
		},
		{
			name:             "rate limited honors retry after",
			status:           http.StatusTooManyRequests,
			body:             `{"success":false,"message":"请求过于频繁"}`,
			retryAfterHeader: "17",
			wantRetryAfter:   17 * time.Second,
		},
		{
			name:               "session revoked",
			status:             http.StatusUnauthorized,
			body:               `{"success":false,"code":"AUTH_SESSION_REVOKED","message":"Unauthorized"}`,
			wantCredentialDead: true,
		},
		{
			name:               "refresh token rejected",
			status:             http.StatusUnauthorized,
			body:               `{"success":false,"code":"AUTH_UNAUTHORIZED","message":"Unauthorized"}`,
			wantCredentialDead: true,
		},
		{
			name:           "concurrent rotation",
			status:         http.StatusConflict,
			body:           `{"success":false,"code":"AUTH_REFRESH_RACE","message":"Conflict"}`,
			wantRetryAfter: newApiV1RefreshRaceBackoff,
		},
		{
			name:   "origin configuration error is not a dead credential",
			status: http.StatusForbidden,
			body:   `{"success":false,"code":"AUTH_ORIGIN_FORBIDDEN","message":"request origin is not allowed"}`,
		},
		{
			name:   "unstructured forbidden is not proof of a dead credential",
			status: http.StatusForbidden,
			body:   `{"success":false,"message":"forbidden by reverse proxy"}`,
		},
		{
			name:               "session mismatch is fatal despite conflict status",
			status:             http.StatusConflict,
			body:               `{"success":false,"code":"AUTH_SESSION_MISMATCH","message":"Conflict"}`,
			wantCredentialDead: true,
		},
		{
			name:   "transient server error",
			status: http.StatusBadGateway,
			body:   `{"success":false,"message":"bad gateway"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if tc.retryAfterHeader != "" {
					w.Header().Set("Retry-After", tc.retryAfterHeader)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			adapter := &NewApiV1Adapter{NewApiAdapter{BaseAdapter: BaseAdapter{Name: "new-api-v1"}}}
			res, err := adapter.RefreshAuth(server.URL, "jwt-old", `{"newApiV1Auth":{"refreshCookie":"old"}}`, nil)
			if err != nil {
				t.Fatalf("HTTP failures must be reported as a result, not an error: %v", err)
			}
			if res == nil || res.Success {
				t.Fatalf("expected failure, got %+v", res)
			}
			if res.RetryAfter != tc.wantRetryAfter {
				t.Fatalf("RetryAfter = %s, want %s", res.RetryAfter, tc.wantRetryAfter)
			}
			if res.CredentialDead != tc.wantCredentialDead {
				t.Fatalf("CredentialDead = %v, want %v", res.CredentialDead, tc.wantCredentialDead)
			}
			if !strings.Contains(res.Message, "refresh request failed") {
				t.Fatalf("message should carry the upstream failure: %q", res.Message)
			}
		})
	}
}

func TestJwtLifetimeMillisFromRefreshedToken(t *testing.T) {
	issuedAt := time.Now().Add(-time.Minute)
	token := newApiV1TestJwt(t, issuedAt, issuedAt.Add(15*time.Minute))
	if got := JwtLifetimeMillis(token); got != int64(15*time.Minute/time.Millisecond) {
		t.Fatalf("JwtLifetimeMillis = %d, want %d", got, int64(15*time.Minute/time.Millisecond))
	}
}

func newApiV1TestJwt(t *testing.T, issuedAt, expiresAt time.Time) string {
	t.Helper()
	payload, err := json.Marshal(map[string]interface{}{"iat": issuedAt.Unix(), "exp": expiresAt.Unix()})
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}

func TestNormalizeEpochMillis(t *testing.T) {
	seconds := int64(1_700_000_000)
	if got := NormalizeEpochMillis(seconds); got != seconds*1000 {
		t.Fatalf("seconds not converted: %d", got)
	}
	millis := int64(1_700_000_000_000)
	if got := NormalizeEpochMillis(millis); got != millis {
		t.Fatalf("millis must pass through: %d", got)
	}
	if got := NormalizeEpochMillis(0); got != 0 {
		t.Fatalf("zero must pass through: %d", got)
	}
}

func jsonNumber(value int64) string {
	out, _ := json.Marshal(value)
	return string(out)
}

func TestSetNewApiV1RefreshCookieAcceptsRawValueOrCookieHeader(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "raw value", input: "  refresh-value  ", want: "refresh-value"},
		{name: "cookie header", input: "session=abc; new_api_refresh=refresh-value; Path=/", want: "refresh-value"},
		{name: "set-cookie header", input: "new_api_refresh=refresh-value; Path=/; HttpOnly", want: "refresh-value"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := map[string]interface{}{}
			SetNewApiV1RefreshCookie(cfg, tc.input)
			node, _ := cfg[NewApiV1AuthConfigKey].(map[string]interface{})
			if node == nil || node[RefreshCookieKey] != tc.want {
				t.Fatalf("stored %#v, want refreshCookie=%q", cfg, tc.want)
			}
		})
	}

	cfg := map[string]interface{}{
		NewApiV1AuthConfigKey: map[string]interface{}{RefreshCookieKey: "old"},
	}
	SetNewApiV1RefreshCookie(cfg, "   ")
	if _, exists := cfg[NewApiV1AuthConfigKey]; exists {
		t.Fatalf("empty input should clear the refresh cookie: %#v", cfg)
	}
}
