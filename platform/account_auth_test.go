package platform

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBaseLoginExtractsTopLevelAccessToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"accessToken":"top-level-token"}`))
	}))
	defer server.Close()

	adapter := &BaseAdapter{Name: "one-api"}
	result, err := adapter.Login(server.URL, "user", "pass", nil)
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if !result.Success || result.AccessToken != "top-level-token" {
		t.Fatalf("unexpected login result: %+v", result)
	}
}

func TestGetAdapterAcceptsLegacyPlatformAliases(t *testing.T) {
	if GetAdapter("newapi") == nil {
		t.Fatal("expected newapi alias to resolve")
	}
	if GetAdapter("oneapi") == nil {
		t.Fatal("expected oneapi alias to resolve")
	}
	if GetAdapter("agent-router") == nil {
		t.Fatal("expected agent-router alias to resolve")
	}
}

func TestOneApiVerifyTokenUsesOneApiBalanceFormula(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/user/self":
			_, _ = w.Write([]byte(`{"success":true,"data":{"username":"alice","quota":1000,"used_quota":200}}`))
		case "/api/token/":
			_, _ = w.Write([]byte(`{"success":true,"data":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	adapter := &OneApiAdapter{BaseAdapter: BaseAdapter{Name: "one-api"}}
	result, err := adapter.VerifyToken(server.URL, "session-token", 0, nil)
	if err != nil {
		t.Fatalf("VerifyToken returned error: %v", err)
	}
	if result.TokenType != "session" || result.Balance == nil {
		t.Fatalf("unexpected verify result: %+v", result)
	}
	if result.Balance.Balance != 0.0016 || result.Balance.Used != 0.0004 || result.Balance.Quota != 0.002 {
		t.Fatalf("unexpected one-api balance: %+v", result.Balance)
	}
}

func TestOneApiVerifyTokenUsesCookieSessionAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Fatalf("cookie session should not be sent as bearer auth: %q", auth)
		}
		if cookie := r.Header.Get("Cookie"); cookie != "session=abc" {
			t.Fatalf("unexpected Cookie header: %q", cookie)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/user/self":
			_, _ = w.Write([]byte(`{"success":true,"data":{"username":"cookie-user","quota":1000,"used_quota":200}}`))
		case "/api/token/":
			_, _ = w.Write([]byte(`{"success":true,"data":[{"name":"main","key":"sk-cookie","status":1}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	adapter := &OneApiAdapter{BaseAdapter: BaseAdapter{Name: "one-api"}}
	result, err := adapter.VerifyToken(server.URL, "session=abc; Path=/; HttpOnly", 0, nil)
	if err != nil {
		t.Fatalf("VerifyToken returned error: %v", err)
	}
	if result.TokenType != "session" || result.UserInfo == nil || result.UserInfo.Username != "cookie-user" {
		t.Fatalf("unexpected verify result: %+v", result)
	}
	if result.ApiToken != "sk-cookie" {
		t.Fatalf("unexpected api token: %q", result.ApiToken)
	}
}

func TestSub2ApiVerifyTokenFetchesUserAndApiToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/auth/me":
			_, _ = w.Write([]byte(`{"code":0,"data":{"username":"bob","email":"bob@example.com","balance":"12.5"}}`))
		case "/api/v1/keys":
			_, _ = w.Write([]byte(`{"code":0,"data":{"items":[{"id":7,"name":"main","key":"sk-sub2","status":"active","group_id":3}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	adapter := &Sub2ApiAdapter{BaseAdapter: BaseAdapter{Name: "sub2api"}}
	result, err := adapter.VerifyToken(server.URL, "jwt-token", 0, nil)
	if err != nil {
		t.Fatalf("VerifyToken returned error: %v", err)
	}
	if result.TokenType != "session" || result.UserInfo == nil || result.UserInfo.Username != "bob" {
		t.Fatalf("unexpected verify result: %+v", result)
	}
	if result.Balance == nil || result.Balance.Balance != 12.5 {
		t.Fatalf("unexpected balance: %+v", result.Balance)
	}
	if result.ApiToken != "sk-sub2" {
		t.Fatalf("unexpected api token: %q", result.ApiToken)
	}
}
