package platform

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthHeadersSendCompatibilityUserIDHeaders(t *testing.T) {
	headers := AuthHeaders("session-token", 42)
	for _, name := range []string{
		"New-API-User",
		"Veloera-User",
		"voapi-user",
		"User-id",
		"Rix-Api-User",
		"neo-api-user",
	} {
		if headers[name] != "42" {
			t.Fatalf("%s = %q", name, headers[name])
		}
	}

	cookieHeaders := CookieUserIDHeaders(42)
	for _, name := range []string{
		"New-Api-User",
		"Veloera-User",
		"voapi-user",
		"User-id",
		"Rix-Api-User",
		"neo-api-user",
	} {
		if cookieHeaders[name] != "42" {
			t.Fatalf("cookie %s = %q", name, cookieHeaders[name])
		}
	}
}

func TestBaseGetApiTokensAcceptsDataArrayWithoutSuccessEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/token/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"key":" sk-onehub ","name":" main ","status":1,"token_group":" vip "}]}`))
	}))
	defer server.Close()

	adapter := &BaseAdapter{Name: "one-api"}
	tokens, err := adapter.GetApiTokens(server.URL, "session-token", 0, nil)
	if err != nil {
		t.Fatalf("GetApiTokens returned error: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("unexpected tokens: %+v", tokens)
	}
	if tokens[0].Key != "sk-onehub" || tokens[0].Name != "main" || tokens[0].TokenGroup != "vip" || !tokens[0].Enabled {
		t.Fatalf("unexpected token normalization: %+v", tokens[0])
	}
}

func TestNewApiGetModelsFallsBackToSessionModelsEndpoint(t *testing.T) {
	sawOpenAIModels := false
	sawSessionModels := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/user/self":
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":11494,"username":"session-user"}}`))
		case "/v1/models":
			sawOpenAIModels = true
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"invalid access token"}`))
		case "/api/user/models":
			sawSessionModels = true
			if got := r.Header.Get("New-API-User"); got != "11494" {
				t.Fatalf("New-API-User = %q", got)
			}
			_, _ = w.Write([]byte(`{"success":true,"data":["gpt-4o","gpt-4.1"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	adapter := &NewApiAdapter{BaseAdapter: BaseAdapter{Name: "new-api"}}
	models, err := adapter.GetModels(server.URL, "session-token", 11494, nil)
	if err != nil {
		t.Fatalf("GetModels returned error: %v", err)
	}
	if !sawOpenAIModels || !sawSessionModels {
		t.Fatalf("expected both model endpoints to be called, saw /v1=%v /api/user/models=%v", sawOpenAIModels, sawSessionModels)
	}
	if len(models) != 2 || models[0] != "gpt-4o" || models[1] != "gpt-4.1" {
		t.Fatalf("unexpected models: %+v", models)
	}
}
