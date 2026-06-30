package platform

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testAgentRouterSessionValue() string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("agentrouter-session", 12)))
	return base64.StdEncoding.EncodeToString([]byte("1782783060|" + payload + "|signature"))
}

func TestAgentRouterVerifyTokenUsesRawSessionCookieAndUserID(t *testing.T) {
	rawSession := testAgentRouterSessionValue()
	wantCookie := "session=" + rawSession

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Fatalf("raw browser session should not be sent as bearer auth: %q", auth)
		}
		if cookie := r.Header.Get("Cookie"); cookie != wantCookie {
			t.Fatalf("unexpected Cookie header:\nwant %q\n got %q", wantCookie, cookie)
		}
		if userID := r.Header.Get("New-API-User"); userID != "12345" {
			t.Fatalf("unexpected New-API-User: %q", userID)
		}

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/user/self":
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":12345,"username":"agent_user","quota":12500000,"used_quota":0}}`))
		case "/api/token/":
			_, _ = w.Write([]byte(`{"success":true,"data":[{"name":"main","key":"sk-agent","status":1}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	adapter := &AgentRouterAdapter{NewApiAdapter: NewApiAdapter{BaseAdapter: BaseAdapter{Name: "agentrouter"}}}
	result, err := adapter.VerifyToken(server.URL, rawSession, 12345, nil)
	if err != nil {
		t.Fatalf("VerifyToken returned error: %v", err)
	}
	if result.TokenType != "session" || result.UserInfo == nil || result.UserInfo.Username != "agent_user" {
		t.Fatalf("unexpected verify result: %+v", result)
	}
	if result.ApiToken != "sk-agent" {
		t.Fatalf("unexpected api token: %q", result.ApiToken)
	}
}

func TestAgentRouterVerifyTokenRequiresUserIDForSession(t *testing.T) {
	rawSession := testAgentRouterSessionValue()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":false,"message":"user id required"}`))
	}))
	defer server.Close()

	adapter := &AgentRouterAdapter{NewApiAdapter: NewApiAdapter{BaseAdapter: BaseAdapter{Name: "agentrouter"}}}
	_, err := adapter.VerifyToken(server.URL, rawSession, 0, nil)
	if err == nil || !strings.Contains(err.Error(), "New-API-User") {
		t.Fatalf("expected New-API-User error, got %v", err)
	}
}

func TestAgentRouterVerifyTokenFallsBackToLogProbeWhenSelfShielded(t *testing.T) {
	rawSession := testAgentRouterSessionValue()
	fullCookie := "acw_tc=stale; session=" + rawSession

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Fatalf("raw browser session should not be sent as bearer auth: %q", auth)
		}
		if userID := r.Header.Get("New-API-User"); userID != "12345" {
			t.Fatalf("unexpected New-API-User: %q", userID)
		}

		switch r.URL.Path {
		case "/api/user/self":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><script>window._shield=1</script></html>`))
		case "/api/log/self":
			if r.Header.Get("Cookie") != "session="+rawSession {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write([]byte(`<html><script>window._shield=1</script></html>`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"data":{"page":1,"page_size":1,"total":0,"items":[{"username":"log_user"}]}}`))
		case "/api/token/":
			if r.Header.Get("Cookie") != "session="+rawSession {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write([]byte(`<html><script>window._shield=1</script></html>`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"data":{"items":[{"name":"main","key":"sk-log","status":1}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	adapter := &AgentRouterAdapter{NewApiAdapter: NewApiAdapter{BaseAdapter: BaseAdapter{Name: "agentrouter"}}}
	result, err := adapter.VerifyToken(server.URL, fullCookie, 12345, nil)
	if err != nil {
		t.Fatalf("VerifyToken returned error: %v", err)
	}
	if result.TokenType != "session" || result.UserInfo == nil || result.UserInfo.Username != "log_user" {
		t.Fatalf("unexpected verify result: %+v", result)
	}
	if result.ApiToken != "sk-log" {
		t.Fatalf("unexpected api token: %q", result.ApiToken)
	}
}

func TestAgentRouterCheckinReadsTodayLog(t *testing.T) {
	rawSession := testAgentRouterSessionValue()
	wantCookie := "session=" + rawSession

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/log/self" {
			http.NotFound(w, r)
			return
		}
		if cookie := r.Header.Get("Cookie"); cookie != wantCookie {
			t.Fatalf("unexpected Cookie header:\nwant %q\n got %q", wantCookie, cookie)
		}
		if userID := r.Header.Get("New-API-User"); userID != "12345" {
			t.Fatalf("unexpected New-API-User: %q", userID)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"page":1,"page_size":20,"total":1,"items":[{"id":993,"user_id":12345,"type":4,"content":"每日签到成功，增加额度 25 额度"}]}}`))
	}))
	defer server.Close()

	adapter := &AgentRouterAdapter{NewApiAdapter: NewApiAdapter{BaseAdapter: BaseAdapter{Name: "agentrouter"}}}
	result, err := adapter.Checkin(server.URL, rawSession, 12345, nil)
	if err != nil {
		t.Fatalf("Checkin returned error: %v", err)
	}
	if result == nil || !result.Success || !strings.Contains(result.Message, "每日签到成功") {
		t.Fatalf("unexpected checkin result: %+v", result)
	}
}
