package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"metapi/aggrsite/db"
	"metapi/aggrsite/platform"
)

// TurnstileSolverConfig holds configuration for the solver service.
type TurnstileSolverConfig struct {
	Provider  string `json:"provider"`  // "yescaptcha", "capsolver", "2captcha", "custom"
	APIKey    string `json:"api_key"`   // Client key / API key
	APIURL    string `json:"api_url"`   // Optional custom API endpoint
	AutoSolve bool   `json:"auto_solve"` // Whether auto-solving is enabled
}

// TurnstileSolver defines the interface for solving Cloudflare Turnstile challenges.
type TurnstileSolver interface {
	SolveTurnstile(ctx context.Context, websiteURL, siteKey string, opt *platform.RequestOption) (string, error)
}

// GetTurnstileSolverConfig loads current configuration from settings table.
func GetTurnstileSolverConfig() TurnstileSolverConfig {
	cfg := TurnstileSolverConfig{
		Provider:  "yescaptcha",
		AutoSolve: true,
	}

	if s, err := db.GetSetting("turnstile_solver_provider"); err == nil && s.Value != nil && *s.Value != "" {
		cfg.Provider = strings.ToLower(SettingStringValue(*s.Value))
	}
	if s, err := db.GetSetting("turnstile_solver_api_key"); err == nil && s.Value != nil {
		cfg.APIKey = SettingStringValue(*s.Value)
	}
	if s, err := db.GetSetting("turnstile_solver_api_url"); err == nil && s.Value != nil {
		cfg.APIURL = SettingStringValue(*s.Value)
	}
	if s, err := db.GetSetting("turnstile_auto_solve"); err == nil && s.Value != nil {
		val := strings.ToLower(SettingStringValue(*s.Value))
		cfg.AutoSolve = val == "1" || val == "true" || val == "yes" || val == "on"
	}

	return cfg
}

// IsTurnstileSolverConfigured returns true if a provider and API key/URL are present.
func IsTurnstileSolverConfigured() bool {
	cfg := GetTurnstileSolverConfig()
	if !cfg.AutoSolve {
		return false
	}
	if cfg.Provider == "custom" || cfg.Provider == "turnstile-solver" {
		return cfg.APIURL != "" || cfg.APIKey != ""
	}
	return cfg.APIKey != ""
}

// NewTurnstileSolver instantiates the appropriate solver from config.
func NewTurnstileSolver(cfg TurnstileSolverConfig) (TurnstileSolver, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = "yescaptcha"
	}

	switch provider {
	case "yescaptcha":
		baseURL := cfg.APIURL
		if baseURL == "" {
			baseURL = "https://api.yescaptcha.com"
		}
		return &standardTaskSolver{
			providerName: "YesCaptcha",
			clientKey:    cfg.APIKey,
			baseURL:      strings.TrimRight(baseURL, "/"),
			taskType:     "TurnstileTaskProxyless",
		}, nil

	case "capsolver":
		baseURL := cfg.APIURL
		if baseURL == "" {
			baseURL = "https://api.capsolver.com"
		}
		return &standardTaskSolver{
			providerName: "CapSolver",
			clientKey:    cfg.APIKey,
			baseURL:      strings.TrimRight(baseURL, "/"),
			taskType:     "AntiTurnstileTaskProxyLess",
		}, nil

	case "2captcha":
		baseURL := cfg.APIURL
		if baseURL == "" {
			baseURL = "https://api.2captcha.com"
		}
		return &standardTaskSolver{
			providerName: "2Captcha",
			clientKey:    cfg.APIKey,
			baseURL:      strings.TrimRight(baseURL, "/"),
			taskType:     "TurnstileTaskProxyless",
		}, nil

	case "custom", "turnstile-solver":
		if cfg.APIURL == "" {
			return nil, errors.New("custom Turnstile solver requires api_url")
		}
		return &customEndpointSolver{
			apiURL:    cfg.APIURL,
			clientKey: cfg.APIKey,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported Turnstile solver provider: %s", provider)
	}
}

// ExtractSiteKeyFromExtraConfig retrieves turnstileSiteKey from extra_config json.
func ExtractSiteKeyFromExtraConfig(extraConfigStr *string) string {
	if extraConfigStr == nil || *extraConfigStr == "" {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(*extraConfigStr), &m); err != nil {
		return ""
	}
	for _, k := range []string{"turnstileSiteKey", "turnstile_site_key", "siteKey", "site_key"} {
		if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// SolveTurnstileIfConfigured attempts to solve Turnstile if configured and siteKey is present.
func SolveTurnstileIfConfigured(websiteURL, siteKey string, opt *platform.RequestOption) (string, error) {
	siteKey = strings.TrimSpace(siteKey)
	if siteKey == "" {
		return "", errors.New("siteKey is empty")
	}

	cfg := GetTurnstileSolverConfig()
	if !cfg.AutoSolve {
		return "", errors.New("Turnstile auto-solve is disabled in settings")
	}

	solver, err := NewTurnstileSolver(cfg)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()

	slog.Info("Solving Turnstile challenge", "provider", cfg.Provider, "website_url", websiteURL)
	token, err := solver.SolveTurnstile(ctx, websiteURL, siteKey, opt)
	if err != nil {
		slog.Error("Turnstile solver failed", "provider", cfg.Provider, "err", err)
		return "", err
	}

	slog.Info("Turnstile challenge solved successfully", "provider", cfg.Provider, "token_len", len(token))
	return token, nil
}

// ---- Standard createTask / getTaskResult Solver (YesCaptcha / CapSolver / 2Captcha) ----

type standardTaskSolver struct {
	providerName string
	clientKey    string
	baseURL      string
	taskType     string
}

type createTaskRequest struct {
	ClientKey string                 `json:"clientKey"`
	Task      map[string]interface{} `json:"task"`
}

type createTaskResponse struct {
	ErrorID          int    `json:"errorId"`
	ErrorCode        string `json:"errorCode,omitempty"`
	ErrorDescription string `json:"errorDescription,omitempty"`
	Status           string `json:"status,omitempty"`
	TaskID           string `json:"taskId,omitempty"`
}

type getTaskResultRequest struct {
	ClientKey string `json:"clientKey"`
	TaskID    string `json:"taskId"`
}

type getTaskResultResponse struct {
	ErrorID          int                    `json:"errorId"`
	ErrorCode        string                 `json:"errorCode,omitempty"`
	ErrorDescription string                 `json:"errorDescription,omitempty"`
	Status           string                 `json:"status"` // "idle", "processing", "ready"
	Solution         map[string]interface{} `json:"solution,omitempty"`
}

func (s *standardTaskSolver) SolveTurnstile(ctx context.Context, websiteURL, siteKey string, opt *platform.RequestOption) (string, error) {
	if s.clientKey == "" {
		return "", fmt.Errorf("%s clientKey is missing", s.providerName)
	}

	client := createHTTPClient(opt)

	// Step 1: Create Task
	createURL := s.baseURL + "/createTask"
	reqPayload := createTaskRequest{
		ClientKey: s.clientKey,
		Task: map[string]interface{}{
			"type":       s.taskType,
			"websiteURL": websiteURL,
			"websiteKey": siteKey,
		},
	}

	reqBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal createTask request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", createURL, bytes.NewReader(reqBytes))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "AggrSite-Turnstile-Client/1.0")

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("%s createTask network error: %w", s.providerName, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("%s failed to read createTask response: %w", s.providerName, err)
	}

	var createRes createTaskResponse
	if err := json.Unmarshal(bodyBytes, &createRes); err != nil {
		return "", fmt.Errorf("%s invalid createTask response (%s): %w", s.providerName, string(bodyBytes), err)
	}

	if createRes.ErrorID != 0 {
		desc := createRes.ErrorDescription
		if desc == "" {
			desc = createRes.ErrorCode
		}
		return "", fmt.Errorf("%s createTask error (code %d): %s", s.providerName, createRes.ErrorID, desc)
	}

	if createRes.TaskID == "" {
		return "", fmt.Errorf("%s createTask returned empty taskId", s.providerName)
	}

	taskID := createRes.TaskID

	// Step 2: Poll getTaskResult
	return s.pollTaskResult(ctx, client, taskID)
}

func (s *standardTaskSolver) pollTaskResult(ctx context.Context, client *http.Client, taskID string) (string, error) {
	getResultURL := s.baseURL + "/getTaskResult"
	pollPayload := getTaskResultRequest{
		ClientKey: s.clientKey,
		TaskID:    taskID,
	}
	pollBytes, _ := json.Marshal(pollPayload)

	// Initial wait
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(3 * time.Second):
	}

	maxAttempts := 30
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		pollReq, err := http.NewRequestWithContext(ctx, "POST", getResultURL, bytes.NewReader(pollBytes))
		if err != nil {
			return "", err
		}
		pollReq.Header.Set("Content-Type", "application/json")
		pollReq.Header.Set("User-Agent", "AggrSite-Turnstile-Client/1.0")

		pollResp, err := client.Do(pollReq)
		if err != nil {
			slog.Warn("Turnstile getTaskResult poll network error", "provider", s.providerName, "attempt", attempt, "err", err)
			time.Sleep(2 * time.Second)
			continue
		}

		pollBody, _ := io.ReadAll(pollResp.Body)
		pollResp.Body.Close()

		var resultRes getTaskResultResponse
		if err := json.Unmarshal(pollBody, &resultRes); err != nil {
			slog.Warn("Turnstile getTaskResult parse error", "provider", s.providerName, "attempt", attempt, "err", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if resultRes.ErrorID != 0 {
			desc := resultRes.ErrorDescription
			if desc == "" {
				desc = resultRes.ErrorCode
			}
			return "", fmt.Errorf("%s getTaskResult error (code %d): %s", s.providerName, resultRes.ErrorID, desc)
		}

		if strings.EqualFold(resultRes.Status, "ready") || strings.EqualFold(resultRes.Status, "success") {
			token := extractTokenFromSolution(resultRes.Solution)
			if token != "" {
				return token, nil
			}
			return "", fmt.Errorf("%s returned status ready but token was not found in solution", s.providerName)
		}

		// Still processing, wait before next attempt
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	return "", fmt.Errorf("%s Turnstile solving timed out after %d attempts", s.providerName, maxAttempts)
}

func extractTokenFromSolution(solution map[string]interface{}) string {
	if solution == nil {
		return ""
	}
	for _, key := range []string{"token", "turnstile_value", "cf_clearance", "value", "response"} {
		if v, ok := solution[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// ---- Custom / Turnstile-Solver Endpoint Solver ----

type customEndpointSolver struct {
	apiURL    string
	clientKey string
}

func (c *customEndpointSolver) SolveTurnstile(ctx context.Context, websiteURL, siteKey string, opt *platform.RequestOption) (string, error) {
	client := createHTTPClient(opt)
	trimmedURL := strings.TrimRight(c.apiURL, "/")

	// Build request payload compatible with multiple self-hosted solver conventions
	payload := map[string]interface{}{
		"url":        websiteURL,
		"sitekey":    siteKey,
		"site_key":   siteKey,
		"websiteURL": websiteURL,
		"websiteKey": siteKey,
	}
	if c.clientKey != "" {
		payload["clientKey"] = c.clientKey
		payload["api_key"] = c.clientKey
	}

	reqBytes, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", trimmedURL, bytes.NewReader(reqBytes))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "AggrSite-Turnstile-Client/1.0")
	if c.clientKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.clientKey)
		httpReq.Header.Set("X-API-Key", c.clientKey)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("custom Turnstile solver network error: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var res map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		// Maybe plain text token
		plain := strings.TrimSpace(string(bodyBytes))
		if len(plain) > 20 && !strings.Contains(plain, "<html") {
			return plain, nil
		}
		return "", fmt.Errorf("custom Turnstile solver returned unparseable response: %s", plain)
	}

	// Check if we got a token directly in the first response
	if token := extractTokenFromCustomResponse(res); token != "" {
		return token, nil
	}

	// Detect async task mode (taozhiyu/Turnstile-Solver style):
	// Response: { "status": "created", "task_id": "uuid" }
	taskID, _ := res["task_id"].(string)
	status, _ := res["status"].(string)
	if taskID != "" && (status == "created" || status == "pending" || status == "processing") {
		slog.Info("Custom solver returned async task, polling for result", "task_id", taskID)
		return c.pollAsyncResult(ctx, client, trimmedURL, taskID)
	}

	// Detect standard createTask response: { "errorId": 0, "taskId": "..." }
	if stdTaskID, ok := res["taskId"].(string); ok && stdTaskID != "" {
		slog.Info("Custom solver returned createTask response, using standardTaskSolver polling", "taskId", stdTaskID)
		std := &standardTaskSolver{
			providerName: "CustomSolver",
			clientKey:    c.clientKey,
			baseURL:      strings.TrimSuffix(trimmedURL, "/createTask"),
			taskType:     "TurnstileTaskProxyless",
		}
		return std.pollTaskResult(ctx, client, stdTaskID)
	}

	return "", fmt.Errorf("could not extract Turnstile token from custom solver response: %s", string(bodyBytes))
}

// pollAsyncResult polls the /result endpoint for taozhiyu/Turnstile-Solver style async tasks.
// API: GET /result?id=<task_id>  →  { "status": "success", "data": { "token": "..." } }
func (c *customEndpointSolver) pollAsyncResult(ctx context.Context, client *http.Client, baseURL, taskID string) (string, error) {
	// Derive result URL: if baseURL is ".../turnstile", result is ".../result"
	resultBaseURL := baseURL
	if idx := strings.LastIndex(resultBaseURL, "/"); idx > 0 {
		resultBaseURL = resultBaseURL[:idx]
	}
	resultURL := resultBaseURL + "/result?id=" + url.QueryEscape(taskID)

	// Initial wait before first poll
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(2 * time.Second):
	}

	maxAttempts := 30
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		pollReq, err := http.NewRequestWithContext(ctx, "GET", resultURL, nil)
		if err != nil {
			return "", err
		}
		pollReq.Header.Set("User-Agent", "AggrSite-Turnstile-Client/1.0")
		if c.clientKey != "" {
			pollReq.Header.Set("Authorization", "Bearer "+c.clientKey)
			pollReq.Header.Set("X-API-Key", c.clientKey)
		}

		pollResp, err := client.Do(pollReq)
		if err != nil {
			slog.Warn("Custom solver poll network error", "attempt", attempt, "err", err)
			time.Sleep(2 * time.Second)
			continue
		}

		pollBody, _ := io.ReadAll(pollResp.Body)
		pollResp.Body.Close()

		var pollRes map[string]interface{}
		if err := json.Unmarshal(pollBody, &pollRes); err != nil {
			slog.Warn("Custom solver poll parse error", "attempt", attempt, "err", err)
			time.Sleep(2 * time.Second)
			continue
		}

		pollStatus, _ := pollRes["status"].(string)
		pollStatus = strings.ToLower(pollStatus)

		if pollStatus == "success" || pollStatus == "ready" {
			if token := extractTokenFromCustomResponse(pollRes); token != "" {
				return token, nil
			}
			return "", fmt.Errorf("custom solver returned status %s but no token found in response: %s", pollStatus, string(pollBody))
		}

		if pollStatus == "error" || pollStatus == "failed" {
			errMsg, _ := pollRes["error"].(string)
			if errMsg == "" {
				errMsg, _ = pollRes["message"].(string)
			}
			return "", fmt.Errorf("custom solver task failed: %s", errMsg)
		}

		// Still pending/processing
		slog.Debug("Custom solver task still pending", "attempt", attempt, "status", pollStatus)
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	return "", fmt.Errorf("custom Turnstile solver timed out after %d poll attempts", maxAttempts)
}

// extractTokenFromCustomResponse extracts a token from various response shapes.
func extractTokenFromCustomResponse(res map[string]interface{}) string {
	// Direct top-level token
	if token := extractTokenFromSolution(res); token != "" {
		return token
	}
	// Nested in "data": { "token": "..." }
	if data, ok := res["data"].(map[string]interface{}); ok {
		if token := extractTokenFromSolution(data); token != "" {
			return token
		}
	}
	// Nested in "solution": { "token": "..." }
	if sol, ok := res["solution"].(map[string]interface{}); ok {
		if token := extractTokenFromSolution(sol); token != "" {
			return token
		}
	}
	return ""
}

// createHTTPClient creates an http.Client configured with optional proxy settings.
func createHTTPClient(opt *platform.RequestOption) *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}

	if opt != nil {
		if opt.ProxyURL != nil && *opt.ProxyURL != "" {
			if proxy, err := url.Parse(*opt.ProxyURL); err == nil {
				transport.Proxy = http.ProxyURL(proxy)
			}
		} else if opt.UseSystemProxy != nil && !*opt.UseSystemProxy {
			transport.Proxy = nil
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}
}
