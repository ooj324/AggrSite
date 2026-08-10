package platform

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newApiV1LoginServer emulates a stateless-auth new-api fork: the login response
// carries a short-lived access token plus a rotating refresh cookie.
func newApiV1LoginServer(t *testing.T, body string, setCookies ...string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/user/self" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":42}}`))
			return
		}
		if r.URL.Path != "/api/user/login" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		for _, cookie := range setCookies {
			w.Header().Add("Set-Cookie", cookie)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

func TestLoginPrefersAccessTokenWhenRefreshCookieIsPresent(t *testing.T) {
	server := newApiV1LoginServer(t,
		`{"success":true,"data":{"access_token":"jwt-token","access_expires_at":1700000000,"user":{"id":42}}}`,
		"new_api_refresh=refresh-value; Path=/; HttpOnly",
	)
	defer server.Close()

	adapter := &NewApiV1Adapter{NewApiAdapter{BaseAdapter: BaseAdapter{Name: "new-api-v1"}}}
	result, err := adapter.Login(server.URL, "user", "pass", nil)
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got message=%q", result.Message)
	}
	if result.AccessToken != "jwt-token" {
		t.Fatalf("AccessToken = %q, want the bearer token (only it is accepted by this fork)", result.AccessToken)
	}
	if result.RefreshCookie != "refresh-value" {
		t.Fatalf("RefreshCookie = %q", result.RefreshCookie)
	}
	if result.ExpiresAt != 1700000000000 {
		t.Fatalf("ExpiresAt = %d, want unix millis", result.ExpiresAt)
	}
	if result.PlatformUserID != 42 {
		t.Fatalf("PlatformUserID = %d, want the id from data.user", result.PlatformUserID)
	}
}

func TestLoginKeepsCookiePreferenceWithoutRefreshCookie(t *testing.T) {
	server := newApiV1LoginServer(t,
		`{"success":true,"data":{"access_token":"jwt-token"}}`,
		"session=session-value; Path=/; HttpOnly",
	)
	defer server.Close()

	adapter := &NewApiAdapter{BaseAdapter: BaseAdapter{Name: "new-api"}}
	result, err := adapter.Login(server.URL, "user", "pass", nil)
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if !strings.Contains(result.AccessToken, "session=session-value") {
		t.Fatalf("AccessToken = %q, want the session cookie for classic new-api", result.AccessToken)
	}
	if result.RefreshCookie != "" {
		t.Fatalf("RefreshCookie = %q, want empty", result.RefreshCookie)
	}
}

func TestLoginFallsBackToCookieWhenBodyHasNoToken(t *testing.T) {
	server := newApiV1LoginServer(t,
		`{"success":true,"data":null}`,
		"new_api_refresh=refresh-value; Path=/; HttpOnly",
	)
	defer server.Close()

	adapter := &NewApiV1Adapter{NewApiAdapter{BaseAdapter: BaseAdapter{Name: "new-api-v1"}}}
	result, err := adapter.Login(server.URL, "user", "pass", nil)
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got message=%q", result.Message)
	}
	if !strings.Contains(result.AccessToken, "new_api_refresh=refresh-value") {
		t.Fatalf("AccessToken = %q, want the cookie jar as last resort", result.AccessToken)
	}
	if result.RefreshCookie != "refresh-value" {
		t.Fatalf("RefreshCookie = %q", result.RefreshCookie)
	}
}

func TestExtractLoginAccessTokenShapes(t *testing.T) {
	cases := []struct {
		name    string
		payload map[string]interface{}
		want    string
	}{
		{
			name:    "classic new-api string data",
			payload: map[string]interface{}{"data": "jwt-from-data"},
			want:    "jwt-from-data",
		},
		{
			name:    "nested access_token wins over nested token",
			payload: map[string]interface{}{"data": map[string]interface{}{"token": "legacy", "access_token": "jwt-nested"}},
			want:    "jwt-nested",
		},
		{
			name:    "top level token",
			payload: map[string]interface{}{"data": map[string]interface{}{}, "token": "jwt-top"},
			want:    "jwt-top",
		},
		{
			name:    "no token",
			payload: map[string]interface{}{"data": nil},
			want:    "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := extractLoginAccessToken(tc.payload); got != tc.want {
				t.Fatalf("extractLoginAccessToken = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildCookieCandidatesOmitsBareRefreshCookie(t *testing.T) {
	candidates := BuildCookieCandidates("session=abc; new_api_refresh=refresh-value")
	for _, candidate := range candidates {
		if candidate == "new_api_refresh=refresh-value" {
			t.Fatalf("refresh cookie must not be offered as an API credential: %v", candidates)
		}
	}
	if len(candidates) == 0 || !strings.Contains(candidates[0], "session=abc") {
		t.Fatalf("unexpected candidates: %v", candidates)
	}
}
