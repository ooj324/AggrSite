package platform

import (
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
