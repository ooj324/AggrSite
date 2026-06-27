package platform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"metapi/aggrsite/config"
	"metapi/aggrsite/db"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// CheckinResult is returned by Checkin calls.
type CheckinResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Reward  string `json:"reward,omitempty"`
}

// BalanceInfo holds balance data from upstream.
type BalanceInfo struct {
	Balance float64 `json:"balance"`
	Used    float64 `json:"used"`
	Quota   float64 `json:"quota"`
}

// LoginResult holds login data from upstream.
type LoginResult struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	AccessToken string `json:"access_token"`
	Username    string `json:"username"`
}

// UserInfo holds user information from upstream.
type UserInfo struct {
	Username    string `json:"username,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Email       string `json:"email,omitempty"`
	Role        int    `json:"role,omitempty"`
}

// VerifyTokenResult holds the verification result.
type VerifyTokenResult struct {
	TokenType string       `json:"tokenType"` // "session", "apikey", "unknown"
	UserInfo  *UserInfo    `json:"userInfo,omitempty"`
	Balance   *BalanceInfo `json:"balance,omitempty"`
	ApiToken  string       `json:"apiToken,omitempty"`
	Models    []string     `json:"models,omitempty"`
}

// ApiTokenInfo holds information about a single upstream API token.
type ApiTokenInfo struct {
	Name       string `json:"name"`
	Key        string `json:"key"`
	Enabled    bool   `json:"enabled"`
	TokenGroup string `json:"tokenGroup,omitempty"`
}

type RequestOption struct {
	ProxyURL       *string
	UseSystemProxy *bool
	CustomHeaders  *string
}

// Adapter is the interface each platform must implement.
type Adapter interface {
	PlatformName() string
	Checkin(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (*CheckinResult, error)
	GetBalance(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (*BalanceInfo, error)
	Login(baseURL, username, password string, opt *RequestOption) (*LoginResult, error)
	GetApiToken(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (string, error)
	GetApiTokens(baseURL, accessToken string, platformUserID int64, opt *RequestOption) ([]ApiTokenInfo, error)
	GetModels(baseURL, accessToken string, platformUserID int64, opt *RequestOption) ([]string, error)
	VerifyToken(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (*VerifyTokenResult, error)
}

// BaseAdapter provides shared HTTP helpers.
type BaseAdapter struct {
	Name string
}

func (b *BaseAdapter) PlatformName() string {
	return b.Name
}

// buildTransport creates an http.Transport with proxy settings from opt.
func buildTransport(opt *RequestOption) *http.Transport {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment, // default
	}

	if opt == nil {
		return transport
	}

	if opt.ProxyURL != nil && *opt.ProxyURL != "" {
		proxy, err := url.Parse(*opt.ProxyURL)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxy)
		}
	} else if opt.UseSystemProxy != nil && *opt.UseSystemProxy {
		systemProxy := config.C.SystemProxyURL
		if dbSetting, err := db.GetSetting("system_proxy_url"); err == nil && dbSetting.Value != nil && *dbSetting.Value != "" {
			systemProxy = strings.Trim(*dbSetting.Value, `"`)
		}
		if systemProxy != "" {
			proxy, err := url.Parse(systemProxy)
			if err == nil {
				transport.Proxy = http.ProxyURL(proxy)
			}
		}
	} else if opt.UseSystemProxy != nil && !*opt.UseSystemProxy {
		transport.Proxy = nil // explicit no proxy
	}

	return transport
}

// applyCustomHeaders parses and applies custom headers from opt onto req.
func applyCustomHeaders(req *http.Request, opt *RequestOption) {
	if opt == nil || opt.CustomHeaders == nil || *opt.CustomHeaders == "" || *opt.CustomHeaders == "{}" {
		return
	}
	var custom map[string]string
	if err := json.Unmarshal([]byte(*opt.CustomHeaders), &custom); err == nil {
		for k, v := range custom {
			req.Header.Set(k, v)
		}
	}
}

// preserveHeadersRedirect preserves sensitive headers like Authorization and Cookie during redirects.
func preserveHeadersRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	for key, val := range via[0].Header {
		req.Header[key] = val
	}
	return nil
}

// FetchJSON makes a JSON request and decodes response.
func (b *BaseAdapter) FetchJSON(reqURL, method string, headers map[string]string, body interface{}, out interface{}, opt *RequestOption) error {
	var bodyReader io.Reader
	if body != nil {
		bs, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(bs)
	}

	req, err := http.NewRequest(method, reqURL, bodyReader)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	
	// Add a default User-Agent to bypass Cloudflare tarpitting of Go-http-client
	hasUA := false
	for k, v := range headers {
		if strings.ToLower(k) == "user-agent" {
			hasUA = true
		}
		req.Header.Set(k, v)
	}
	if !hasUA {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	}
	
	applyCustomHeaders(req, opt)

	client := &http.Client{
		Timeout:       60 * time.Second,
		Transport:     buildTransport(opt),
		CheckRedirect: preserveHeadersRedirect,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Attempt to extract a cleaner message from JSON error body
		var errData map[string]interface{}
		if jsonErr := json.Unmarshal(respBody, &errData); jsonErr == nil {
			if msg := ExtractMessage(errData); msg != "" {
				return fmt.Errorf("HTTP %d: %s", resp.StatusCode, msg)
			}
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	if out != nil {
		return json.Unmarshal(respBody, out)
	}
	return nil
}

// AuthHeaders builds common user-id headers used by new-api / one-api family.
func AuthHeaders(accessToken string, platformUserID int64) map[string]string {
	cleanToken := strings.TrimSpace(accessToken)
	cleanToken = strings.TrimPrefix(cleanToken, "Bearer ")
	cleanToken = strings.TrimSpace(cleanToken)

	h := map[string]string{
		"Authorization": "Bearer " + cleanToken,
	}
	if platformUserID > 0 {
		uid := fmt.Sprintf("%d", platformUserID)
		h["New-API-User"] = uid
		h["Veloera-User"] = uid
		h["User-id"] = uid
	}
	return h
}

// ExtractMessage extracts a human-readable message from a typical API response body.
func ExtractMessage(data map[string]interface{}) string {
	if msg, ok := data["message"].(string); ok && strings.TrimSpace(msg) != "" {
		return strings.TrimSpace(msg)
	}
	if msg, ok := data["msg"].(string); ok && strings.TrimSpace(msg) != "" {
		return strings.TrimSpace(msg)
	}
	if errObj, ok := data["error"].(map[string]interface{}); ok {
		if msg, ok := errObj["message"].(string); ok && strings.TrimSpace(msg) != "" {
			return strings.TrimSpace(msg)
		}
	}
	return ""
}

// Login performs a standard username/password login for new-api compatible platforms.
func (b *BaseAdapter) Login(baseURL, username, password string, opt *RequestOption) (*LoginResult, error) {
	url := fmt.Sprintf("%s/api/user/login", baseURL)
	body := map[string]string{
		"username": username,
		"password": password,
	}

	var res map[string]interface{}
	err := b.FetchJSON(url, "POST", nil, body, &res, opt)
	if err != nil {
		return &LoginResult{Success: false, Message: err.Error()}, nil
	}

	success, _ := res["success"].(bool)
	message := ExtractMessage(res)
	
	if !success {
		if message == "" {
			message = "Login failed"
		}
		return &LoginResult{Success: false, Message: message}, nil
	}

	data := res["data"]
	var token string
	if s, ok := data.(string); ok {
		token = s
	} else if m, ok := data.(map[string]interface{}); ok {
		if t, ok := m["token"].(string); ok {
			token = t
		} else if t, ok := m["access_token"].(string); ok {
			token = t
		}
	}

	if token == "" {
		return &LoginResult{Success: false, Message: "No token in response"}, nil
	}

	return &LoginResult{
		Success:     true,
		AccessToken: token,
		Username:    username,
	}, nil
}

// GetApiToken fetches the first enabled API token.
func (b *BaseAdapter) GetApiToken(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (string, error) {
	tokens, err := b.GetApiTokens(baseURL, accessToken, platformUserID, opt)
	if err != nil {
		return "", err
	}
	for _, t := range tokens {
		if t.Enabled && t.Key != "" {
			return t.Key, nil
		}
	}
	if len(tokens) > 0 && tokens[0].Key != "" {
		return tokens[0].Key, nil
	}
	return "", fmt.Errorf("no valid api token found")
}

func (b *BaseAdapter) GetApiTokens(baseURL, accessToken string, platformUserID int64, opt *RequestOption) ([]ApiTokenInfo, error) {
	url := fmt.Sprintf("%s/api/token/?p=0&size=100", baseURL)
	headers := AuthHeaders(accessToken, platformUserID)

	var res map[string]interface{}
	err := b.FetchJSON(url, "GET", headers, nil, &res, opt)
	if err != nil {
		return nil, err
	}

	success, _ := res["success"].(bool)
	if !success {
		return nil, fmt.Errorf("fetch token failed: %s", ExtractMessage(res))
	}

	data, _ := res["data"].([]interface{})
	var tokens []ApiTokenInfo
	for i, item := range data {
		if m, ok := item.(map[string]interface{}); ok {
			enabled := false
			if status, ok := m["status"].(float64); ok && status == 1 {
				enabled = true
			} else if e, ok := m["enabled"].(bool); ok && e {
				enabled = true
			} else if _, hasStatus := m["status"]; !hasStatus {
				if _, hasEnabled := m["enabled"]; !hasEnabled {
					enabled = true
				}
			}
			
			key, _ := m["key"].(string)
			name, _ := m["name"].(string)
			if name == "" {
				name = fmt.Sprintf("token-%d", i+1)
			}
			group := ""
			if g, ok := m["group"].(string); ok {
				group = g
			} else if gn, ok := m["group_name"].(string); ok {
				group = gn
			} else if tg, ok := m["token_group"].(string); ok {
				group = tg
			}

			if key != "" {
				tokens = append(tokens, ApiTokenInfo{
					Name:       name,
					Key:        key,
					Enabled:    enabled,
					TokenGroup: group,
				})
			}
		}
	}

	return tokens, nil
}

func (b *BaseAdapter) GetModels(baseURL, accessToken string, platformUserID int64, opt *RequestOption) ([]string, error) {
	url := fmt.Sprintf("%s/v1/models", baseURL)
	headers := AuthHeaders(accessToken, platformUserID)

	var res map[string]interface{}
	err := b.FetchJSON(url, "GET", headers, nil, &res, opt)
	if err != nil {
		return nil, err
	}

	var models []string
	if data, ok := res["data"].([]interface{}); ok {
		for _, item := range data {
			if m, ok := item.(map[string]interface{}); ok {
				if id, ok := m["id"].(string); ok && id != "" {
					models = append(models, id)
				}
			}
		}
		return models, nil
	}

	return nil, fmt.Errorf("no models array in response")
}

func (b *BaseAdapter) VerifyToken(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (*VerifyTokenResult, error) {
	// 1. Try session validation: /api/user/self
	userURL := fmt.Sprintf("%s/api/user/self", baseURL)
	headers := AuthHeaders(accessToken, platformUserID)
	
	var userRes map[string]interface{}
	err := b.FetchJSON(userURL, "GET", headers, nil, &userRes, opt)
	if err == nil {
		success, _ := userRes["success"].(bool)
		if success {
			var ui *UserInfo
			var bi *BalanceInfo
			if data, ok := userRes["data"].(map[string]interface{}); ok && data != nil {
				ui = &UserInfo{
					Username:    fmt.Sprintf("%v", data["username"]),
					DisplayName: fmt.Sprintf("%v", data["display_name"]),
					Email:       fmt.Sprintf("%v", data["email"]),
				}
				if r, ok := data["role"].(float64); ok {
					ui.Role = int(r)
				}
				
				divisor := 500000.0
				quota := 0.0
				used := 0.0
				if q, ok := data["quota"].(float64); ok {
					quota = q / divisor
				}
				if u, ok := data["used_quota"].(float64); ok {
					used = u / divisor
				}
				bi = &BalanceInfo{
					Balance: quota,
					Used:    used,
					Quota:   quota + used,
				}
			}

			apiToken, _ := b.GetApiToken(baseURL, accessToken, platformUserID, opt)

			return &VerifyTokenResult{
				TokenType: "session",
				UserInfo:  ui,
				Balance:   bi,
				ApiToken:  apiToken,
			}, nil
		}
	}

	// 2. Try apikey validation: /v1/models
	models, err := b.GetModels(baseURL, accessToken, platformUserID, opt)
	if err == nil && len(models) > 0 {
		return &VerifyTokenResult{
			TokenType: "apikey",
			Models:    models,
		}, nil
	}

	return &VerifyTokenResult{
		TokenType: "unknown",
	}, nil
}
