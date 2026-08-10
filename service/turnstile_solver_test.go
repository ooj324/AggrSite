package service

import (
	"context"
	"encoding/json"
	"metapi/aggrsite/config"
	"metapi/aggrsite/db"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTurnstileSolver_ConfigAndValidation(t *testing.T) {
	if db.DB != nil {
		_ = db.DB.Close()
		db.DB = nil
	}
	tmpDir := t.TempDir()
	t.Setenv("DB_URL", filepath.Join(tmpDir, "test.db"))
	config.Init()
	db.Init()
	t.Cleanup(func() {
		if db.DB != nil {
			_ = db.DB.Close()
			db.DB = nil
		}
		_ = os.Remove(config.C.DBUrl)
	})

	_ = db.UpsertSetting("turnstile_solver_provider", "yescaptcha")
	_ = db.UpsertSetting("turnstile_solver_api_key", "test-client-key-123")
	_ = db.UpsertSetting("turnstile_auto_solve", "1")

	cfg := GetTurnstileSolverConfig()
	if cfg.Provider != "yescaptcha" {
		t.Fatalf("expected provider yescaptcha, got %s", cfg.Provider)
	}
	if cfg.APIKey != "test-client-key-123" {
		t.Fatalf("expected apiKey test-client-key-123, got %s", cfg.APIKey)
	}
	if !cfg.AutoSolve {
		t.Fatalf("expected autoSolve true")
	}
	if !IsTurnstileSolverConfigured() {
		t.Fatalf("expected IsTurnstileSolverConfigured to be true")
	}

	solver, err := NewTurnstileSolver(cfg)
	if err != nil {
		t.Fatalf("NewTurnstileSolver failed: %v", err)
	}
	if solver == nil {
		t.Fatalf("expected non-nil solver")
	}
}

func TestTurnstileSolver_StandardTaskFlow(t *testing.T) {
	// Mock YesCaptcha / CapSolver server
	createTaskCalled := false
	getTaskResultCalled := false

	mockSolverServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/createTask" {
			createTaskCalled = true
			var req createTaskRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.ClientKey != "test-key" {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"errorId":          1,
					"errorDescription": "invalid clientKey",
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errorId": 0,
				"status":  "processing",
				"taskId":  "task-abc-123",
			})
			return
		}

		if r.URL.Path == "/getTaskResult" {
			getTaskResultCalled = true
			var req getTaskResultRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.TaskID == "task-abc-123" {
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"errorId": 0,
					"status":  "ready",
					"solution": map[string]interface{}{
						"token": "turnstile-token-mock-xyz",
					},
				})
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errorId":          2,
				"errorDescription": "task not found",
			})
			return
		}

		http.NotFound(w, r)
	}))
	defer mockSolverServer.Close()

	cfg := TurnstileSolverConfig{
		Provider:  "yescaptcha",
		APIKey:    "test-key",
		APIURL:    mockSolverServer.URL,
		AutoSolve: true,
	}

	solver, err := NewTurnstileSolver(cfg)
	if err != nil {
		t.Fatalf("failed to create solver: %v", err)
	}

	token, err := solver.SolveTurnstile(context.Background(), "https://example.com", "0x4AAAAAAtest", nil)
	if err != nil {
		t.Fatalf("SolveTurnstile failed: %v", err)
	}

	if token != "turnstile-token-mock-xyz" {
		t.Fatalf("expected token 'turnstile-token-mock-xyz', got %q", token)
	}

	if !createTaskCalled || !getTaskResultCalled {
		t.Fatalf("expected both createTask and getTaskResult to be called (create=%v, get=%v)", createTaskCalled, getTaskResultCalled)
	}
}

func TestTurnstileSolver_FallsBackToSecondInstance(t *testing.T) {
	firstCalls := 0
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstCalls++
		http.Error(w, "instance unavailable", http.StatusServiceUnavailable)
	}))
	defer first.Close()

	secondCalls := 0
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondCalls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"token": "token-from-second-instance",
		})
	}))
	defer second.Close()

	solver, err := NewTurnstileSolver(TurnstileSolverConfig{
		Provider: "custom",
		APIURL:   first.URL + "\n" + second.URL,
	})
	if err != nil {
		t.Fatalf("NewTurnstileSolver failed: %v", err)
	}

	token, err := solver.SolveTurnstile(context.Background(), "https://example.com", "site-key", nil)
	if err != nil {
		t.Fatalf("expected second instance to succeed, got %v", err)
	}
	if token != "token-from-second-instance" {
		t.Fatalf("expected token from second instance, got %q", token)
	}
	if firstCalls != 1 || secondCalls != 1 {
		t.Fatalf("expected both instances to be called once, got first=%d second=%d", firstCalls, secondCalls)
	}
}

func TestTurnstileSolver_ReturnsAllInstanceErrors(t *testing.T) {
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "first unavailable", http.StatusServiceUnavailable)
	}))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "second unavailable", http.StatusBadGateway)
	}))
	defer second.Close()

	solver, err := NewTurnstileSolver(TurnstileSolverConfig{
		Provider: "custom",
		APIURL:   first.URL + "\r\n\r\n" + second.URL,
	})
	if err != nil {
		t.Fatalf("NewTurnstileSolver failed: %v", err)
	}

	_, err = solver.SolveTurnstile(context.Background(), "https://example.com", "site-key", nil)
	if err == nil {
		t.Fatal("expected all instances to fail")
	}
	for _, want := range []string{"all Turnstile solver instances failed", "instance 1", "instance 2"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error to contain %q, got %v", want, err)
		}
	}
}

func TestCheckinAccount_TurnstileAutoSolved(t *testing.T) {
	if db.DB != nil {
		_ = db.DB.Close()
		db.DB = nil
	}
	tmpDir := t.TempDir()
	t.Setenv("DB_URL", filepath.Join(tmpDir, "test.db"))
	config.Init()
	db.Init()
	t.Cleanup(func() {
		if db.DB != nil {
			_ = db.DB.Close()
			db.DB = nil
		}
		_ = os.Remove(config.C.DBUrl)
	})

	// Mock solver server
	mockSolverServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/createTask" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errorId": 0,
				"status":  "processing",
				"taskId":  "task-turnstile-1",
			})
			return
		}
		if r.URL.Path == "/getTaskResult" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"errorId": 0,
				"status":  "ready",
				"solution": map[string]interface{}{
					"token": "valid-cf-token-12345",
				},
			})
			return
		}
	}))
	defer mockSolverServer.Close()

	// Configure solver settings in DB
	_ = db.UpsertSetting("turnstile_solver_provider", "yescaptcha")
	_ = db.UpsertSetting("turnstile_solver_api_key", "valid-key")
	_ = db.UpsertSetting("turnstile_solver_api_url", mockSolverServer.URL)
	_ = db.UpsertSetting("turnstile_auto_solve", "true")

	// Mock target upstream API server
	checkinAttempts := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.URL.Path == "/api/user/self" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"id":         1,
					"username":   "alice",
					"quota":      500000,
					"used_quota": 0,
				},
			})
			return
		}

		if r.URL.Path == "/api/user/sign_in" || r.URL.Path == "/api/user/checkin" {
			checkinAttempts++
			turnstileParam := r.URL.Query().Get("turnstile")
			if turnstileParam == "" {
				var body map[string]interface{}
				_ = json.NewDecoder(r.Body).Decode(&body)
				if val, ok := body["turnstile"].(string); ok {
					turnstileParam = val
				}
			}

			if turnstileParam == "valid-cf-token-12345" {
				// Turnstile challenge passed
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": true,
					"message": "签到成功",
					"data": map[string]interface{}{
						"reward": 500,
					},
				})
				return
			}

			// First request without valid turnstile token
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Turnstile check failed or required",
			})
			return
		}

		http.NotFound(w, r)
	}))
	defer upstream.Close()

	siteKey := "0x4AAAAAA_test_sitekey"
	siteID, err := db.CreateSite(db.CreateSiteInput{
		Name:             "Test Site",
		URL:              upstream.URL,
		Platform:         "new-api",
		Status:           "active",
		TurnstileSiteKey: &siteKey,
	})
	if err != nil {
		t.Fatalf("CreateSite failed: %v", err)
	}

	accID, err := db.CreateAccount(db.CreateAccountInput{
		SiteID:         siteID,
		Username:       "testuser",
		AccessToken:    "session=cookie_val_123",
		CheckinEnabled: true,
		Status:         "active",
		CredentialMode: "session",
	})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	// Perform checkin
	res, err := CheckinAccount(accID)
	if err != nil {
		t.Fatalf("CheckinAccount returned unexpected error: %v", err)
	}

	if !res.Success {
		t.Fatalf("expected checkin to succeed via auto Turnstile solver, got status=%s, message=%s", res.Status, res.Message)
	}
	if res.Status != "success" {
		t.Fatalf("expected status 'success', got %q", res.Status)
	}
	if checkinAttempts < 2 {
		t.Fatalf("expected at least 2 checkin attempts (initial + retry after solving), got %d", checkinAttempts)
	}
}

func TestTurnstileSolver_TaozhiyuAsyncFlow(t *testing.T) {
	// Mock taozhiyu/Turnstile-Solver API:
	// POST /turnstile -> 202 { "status": "created", "task_id": "uuid-123" }
	// GET /result?id=uuid-123 -> 200 { "status": "success", "data": { "token": "camoufox-solved-token-999" } }
	postCalled := false
	getResultCalled := false

	mockSolver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" && (r.URL.Path == "/turnstile" || r.URL.Path == "/solve") {
			postCalled = true
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["sitekey"] == "" && body["websiteKey"] == "" {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "error": "missing sitekey"})
				return
			}
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "created",
				"task_id": "mock-task-uuid-456",
			})
			return
		}

		if r.Method == "GET" && r.URL.Path == "/result" {
			getResultCalled = true
			taskID := r.URL.Query().Get("id")
			if taskID == "mock-task-uuid-456" {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status": "success",
					"data": map[string]interface{}{
						"token":        "camoufox-solved-token-999",
						"elapsed_time": 3.12,
					},
				})
				return
			}
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "error": "invalid task id"})
			return
		}

		http.NotFound(w, r)
	}))
	defer mockSolver.Close()

	cfg := TurnstileSolverConfig{
		Provider:  "custom",
		APIURL:    mockSolver.URL + "/turnstile",
		AutoSolve: true,
	}

	solver, err := NewTurnstileSolver(cfg)
	if err != nil {
		t.Fatalf("NewTurnstileSolver failed: %v", err)
	}

	token, err := solver.SolveTurnstile(context.Background(), "https://my-site.com", "0x4AAAAAAtestkey", nil)
	if err != nil {
		t.Fatalf("SolveTurnstile with taozhiyu async flow failed: %v", err)
	}

	if token != "camoufox-solved-token-999" {
		t.Fatalf("expected 'camoufox-solved-token-999', got %q", token)
	}

	if !postCalled || !getResultCalled {
		t.Fatalf("expected POST /turnstile and GET /result to be called (post=%v, get=%v)", postCalled, getResultCalled)
	}
}
