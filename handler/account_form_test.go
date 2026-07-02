package handler

import (
	"bytes"
	"encoding/json"
	"metapi/aggrsite/db"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func callJSONRoute(t *testing.T, method, path string, route func(chi.Router), payload map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal payload failed: %v", err)
	}

	router := chi.NewRouter()
	route(router)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func responseDataMap(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var envelope map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("Unmarshal response failed: %v", err)
	}
	data, ok := envelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("response missing data object: %#v", envelope)
	}
	return data
}

func accountExtraConfig(t *testing.T, account *db.Account) map[string]interface{} {
	t.Helper()

	var cfg map[string]interface{}
	if account.ExtraConfig != nil && strings.TrimSpace(*account.ExtraConfig) != "" {
		if err := json.Unmarshal([]byte(*account.ExtraConfig), &cfg); err != nil {
			t.Fatalf("invalid extra_config: %v", err)
		}
	}
	if cfg == nil {
		cfg = make(map[string]interface{})
	}
	return cfg
}

func TestCreateAccountTrimsApiKeyAndPreservesSubmittedStatus(t *testing.T) {
	siteID := setupVerifyTokenTestDB(t, "https://example.invalid")

	rec := callJSONRoute(t, http.MethodPost, "/api/accounts", func(r chi.Router) {
		r.Post("/api/accounts", CreateAccount)
	}, map[string]interface{}{
		"site_id":            siteID,
		"username":           "  api key account  ",
		"access_token":       "  sk-live\n",
		"credentialMode":     "apikey",
		"platformUserId":     0,
		"status":             "disabled",
		"checkin_enabled":    true,
		"skipModelFetch":     true,
		"useSystemProxy":     false,
		"checkin_credential": "  ",
	})
	data := responseDataMap(t, rec)

	id := int64(data["id"].(float64))
	account, err := db.GetAccount(id)
	if err != nil {
		t.Fatalf("GetAccount failed: %v", err)
	}
	if account.AccessToken != "" {
		t.Fatalf("AccessToken = %q", account.AccessToken)
	}
	if account.ApiToken == nil || *account.ApiToken != "sk-live" {
		t.Fatalf("ApiToken = %v", account.ApiToken)
	}
	if account.Username == nil || *account.Username != "api key account" {
		t.Fatalf("Username = %v", account.Username)
	}
	if account.Status == nil || *account.Status != "disabled" {
		t.Fatalf("Status = %v", account.Status)
	}
	if _, exists := accountExtraConfig(t, account)["platformUserId"]; exists {
		t.Fatalf("platformUserId should not be stored for zero input: %s", *account.ExtraConfig)
	}
}

func TestUpdateAccountClearsEmptyExtraConfigFieldsAndTrimsToken(t *testing.T) {
	siteID := setupVerifyTokenTestDB(t, "https://example.invalid")
	id, err := db.CreateAccount(db.CreateAccountInput{
		SiteID:         siteID,
		Username:       "session account",
		AccessToken:    "old-session",
		CheckinEnabled: true,
		Status:         "active",
		CredentialMode: "session",
	})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}
	extraConfig := `{"credentialMode":"session","platformUserId":123,"proxyUrl":"http://127.0.0.1:7890","checkin_credential":"cookie=old","sub2apiAuth":{"refreshToken":"old-refresh","tokenExpiresAt":12345}}`
	if err := db.UpdateAccount(id, map[string]interface{}{"extra_config": extraConfig}); err != nil {
		t.Fatalf("UpdateAccount setup failed: %v", err)
	}

	rec := callJSONRoute(t, http.MethodPut, "/api/accounts/"+strconv.FormatInt(id, 10), func(r chi.Router) {
		r.Put("/api/accounts/{id}", UpdateAccount)
	}, map[string]interface{}{
		"access_token":       "  session-new\n",
		"credentialMode":     "session",
		"platformUserId":     nil,
		"proxyUrl":           "",
		"checkin_credential": "",
		"refreshToken":       "",
		"tokenExpiresAt":     nil,
	})
	responseDataMap(t, rec)

	account, err := db.GetAccount(id)
	if err != nil {
		t.Fatalf("GetAccount failed: %v", err)
	}
	if account.AccessToken != "session-new" {
		t.Fatalf("AccessToken = %q", account.AccessToken)
	}
	cfg := accountExtraConfig(t, account)
	for _, key := range []string{"platformUserId", "proxyUrl", "checkin_credential", "sub2apiAuth"} {
		if _, exists := cfg[key]; exists {
			t.Fatalf("%s should be cleared from extra_config: %#v", key, cfg)
		}
	}
}

func TestRebindSessionClearsZeroPlatformUserIDAndTrimsToken(t *testing.T) {
	sawTrimmedAuth := false
	sawOldUserIDHeader := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/user/self" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") == "Bearer session-new" {
			sawTrimmedAuth = true
		}
		if r.Header.Get("New-API-User") == "123" || r.Header.Get("New-Api-User") == "123" {
			sawOldUserIDHeader = true
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":1,"username":"alice","quota":1000,"used_quota":0}}`))
	}))
	defer upstream.Close()

	siteID := setupVerifyTokenTestDB(t, upstream.URL)
	id, err := db.CreateAccount(db.CreateAccountInput{
		SiteID:         siteID,
		Username:       "session account",
		AccessToken:    "old-session",
		CheckinEnabled: true,
		Status:         "active",
		CredentialMode: "session",
	})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}
	extraConfig := `{"credentialMode":"session","platformUserId":123,"sub2apiAuth":{"refreshToken":"old-refresh","tokenExpiresAt":12345}}`
	if err := db.UpdateAccount(id, map[string]interface{}{"extra_config": extraConfig}); err != nil {
		t.Fatalf("UpdateAccount setup failed: %v", err)
	}

	rec := callJSONRoute(t, http.MethodPost, "/api/accounts/"+strconv.FormatInt(id, 10)+"/rebind-session", func(r chi.Router) {
		r.Post("/api/accounts/{id}/rebind-session", RebindSession)
	}, map[string]interface{}{
		"accessToken":    "  session-new\n",
		"platformUserId": 0,
		"refreshToken":   "",
		"tokenExpiresAt": 0,
	})
	responseDataMap(t, rec)

	account, err := db.GetAccount(id)
	if err != nil {
		t.Fatalf("GetAccount failed: %v", err)
	}
	if account.AccessToken != "session-new" {
		t.Fatalf("AccessToken = %q", account.AccessToken)
	}
	cfg := accountExtraConfig(t, account)
	if _, exists := cfg["platformUserId"]; exists {
		t.Fatalf("platformUserId should be cleared: %#v", cfg)
	}
	if _, exists := cfg["sub2apiAuth"]; exists {
		t.Fatalf("sub2apiAuth should be cleared: %#v", cfg)
	}
	if !sawTrimmedAuth {
		t.Fatalf("upstream did not receive trimmed bearer token")
	}
	if sawOldUserIDHeader {
		t.Fatalf("old platformUserId was forwarded as a user id header")
	}
}
