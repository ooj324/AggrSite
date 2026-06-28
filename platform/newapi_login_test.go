package platform

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewApiLoginAcceptsUsableSetCookieSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/user/login" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Add("Set-Cookie", "session=session-value; Path=/; HttpOnly")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":null}`))
	}))
	defer server.Close()

	adapter := &NewApiAdapter{BaseAdapter: BaseAdapter{Name: "new-api"}}
	result, err := adapter.Login(server.URL, "user", "pass", nil)
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected login success, got message=%q", result.Message)
	}
	if !strings.Contains(result.AccessToken, "session=session-value") {
		t.Fatalf("expected session cookie access token, got %q", result.AccessToken)
	}
}

func TestNewApiLoginIgnoresShieldOnlyCookie(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Set-Cookie", "cdn_sec_tc=shield-cookie; Path=/; HttpOnly")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":null}`))
	}))
	defer server.Close()

	adapter := &NewApiAdapter{BaseAdapter: BaseAdapter{Name: "new-api"}}
	result, err := adapter.Login(server.URL, "user", "pass", nil)
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if result.Success {
		t.Fatalf("expected shield-only cookie login failure, got token=%q", result.AccessToken)
	}
}

func TestParseApiTokensArraySupportsNestedPayloads(t *testing.T) {
	payload := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"key": "sk-enabled", "status": float64(1)},
				map[string]interface{}{"name": "disabled", "key": "sk-disabled", "status": float64(0)},
			},
		},
	}

	tokens := parseApiTokensArray(payload)
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	if tokens[0].Name != "default" || tokens[0].Key != "sk-enabled" || !tokens[0].Enabled {
		t.Fatalf("unexpected first token: %+v", tokens[0])
	}
	if tokens[1].Name != "disabled" || tokens[1].Enabled {
		t.Fatalf("unexpected second token: %+v", tokens[1])
	}
}
