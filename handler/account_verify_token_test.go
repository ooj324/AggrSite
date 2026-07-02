package handler

import (
	"bytes"
	"encoding/json"
	"metapi/aggrsite/config"
	"metapi/aggrsite/db"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func setupVerifyTokenTestDB(t *testing.T, upstreamURL string) int64 {
	t.Helper()

	if db.DB != nil {
		_ = db.DB.Close()
		db.DB = nil
	}

	t.Setenv("DB_URL", filepath.Join(t.TempDir(), "aggrsite-test.db"))
	config.Init()
	db.Init()

	db.DB.MustExec(`CREATE TABLE IF NOT EXISTS sites (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		url TEXT NOT NULL,
		platform TEXT NOT NULL,
		status TEXT DEFAULT 'active',
		created_at TEXT,
		updated_at TEXT,
		is_pinned INTEGER DEFAULT 0,
		sort_order INTEGER DEFAULT 0,
		proxy_url TEXT,
		use_system_proxy INTEGER DEFAULT 0,
		custom_headers TEXT,
		external_checkin_url TEXT,
		external_checkin_method TEXT,
		external_checkin_auth_header TEXT,
		external_checkin_auth_prefix TEXT,
		external_checkin_body TEXT
	)`)

	id, err := db.CreateSite(db.CreateSiteInput{
		Name:     "Verify Site",
		URL:      upstreamURL,
		Platform: "new-api",
		Status:   "active",
	})
	if err != nil {
		t.Fatalf("CreateSite failed: %v", err)
	}
	t.Cleanup(func() {
		if db.DB != nil {
			_ = db.DB.Close()
			db.DB = nil
		}
		_ = os.Remove(config.C.DBUrl)
	})
	return id
}

func callVerifyToken(t *testing.T, payload map[string]interface{}) map[string]interface{} {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal payload failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/accounts/verify-token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	VerifyToken(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("VerifyToken status = %d, body = %s", rec.Code, rec.Body.String())
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

func TestVerifyTokenTrimsAccessTokenBeforeVerification(t *testing.T) {
	sawTrimmedAuth := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/user/self" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Authorization"); got == "Bearer session-token" {
			sawTrimmedAuth = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":1,"username":"alice","quota":1000,"used_quota":500}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":false,"message":"bad auth"}`))
	}))
	defer upstream.Close()

	siteID := setupVerifyTokenTestDB(t, upstream.URL)
	result := callVerifyToken(t, map[string]interface{}{
		"siteId":         siteID,
		"accessToken":    "  session-token\n",
		"credentialMode": "session",
	})

	if result["success"] != true {
		t.Fatalf("expected success result, got %#v", result)
	}
	if result["tokenType"] != "session" {
		t.Fatalf("tokenType = %v", result["tokenType"])
	}
	if !sawTrimmedAuth {
		t.Fatalf("upstream did not receive trimmed bearer token")
	}
}

func TestVerifyTokenRejectsEmptyAccessTokenAsBusinessFailure(t *testing.T) {
	siteID := setupVerifyTokenTestDB(t, "https://example.invalid")
	result := callVerifyToken(t, map[string]interface{}{
		"siteId":      siteID,
		"accessToken": " \n\t",
	})

	if result["success"] != false {
		t.Fatalf("expected business failure, got %#v", result)
	}
	if result["message"] != "Token 不能为空" {
		t.Fatalf("message = %v", result["message"])
	}
}

func TestVerifyTokenApiKeyModeReturnsBusinessFailureOnNoModels(t *testing.T) {
	sawUserIDHeader := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("New-Api-User") != "" || r.Header.Get("New-API-User") != "" {
			sawUserIDHeader = true
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer upstream.Close()

	siteID := setupVerifyTokenTestDB(t, upstream.URL)
	result := callVerifyToken(t, map[string]interface{}{
		"siteId":         siteID,
		"accessToken":    "sk-empty",
		"credentialMode": "apikey",
		"platformUserId": 0,
	})

	if result["success"] != false {
		t.Fatalf("expected business failure, got %#v", result)
	}
	if result["message"] != "API Key 验证失败：未获取到可用模型" {
		t.Fatalf("message = %v", result["message"])
	}
	if sawUserIDHeader {
		t.Fatalf("platformUserId 0 was forwarded as a user id header")
	}
}
