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
	Success        bool   `json:"success"`
	Message        string `json:"message"`
	AccessToken    string `json:"access_token"`
	Username       string `json:"username"`
	PlatformUserID int64  `json:"platform_user_id"`
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

type RefreshResult struct {
	Success     bool
	AccessToken string
	ExtraConfig string // Merged or updated extraConfig
	Message     string
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
	RefreshAuth(baseURL, accessToken, extraConfig string, opt *RequestOption) (*RefreshResult, error)
}

// BaseAdapter provides shared HTTP helpers.
type BaseAdapter struct {
	Name string
}

func (b *BaseAdapter) PlatformName() string {
	return b.Name
}

// RefreshAuth default implementation: returns not supported.
func (b *BaseAdapter) RefreshAuth(baseURL, accessToken, extraConfig string, opt *RequestOption) (*RefreshResult, error) {
	return &RefreshResult{Success: false, Message: "RefreshAuth is not supported by " + b.Name}, nil
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

func stripBearerPrefix(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) >= 7 && strings.EqualFold(trimmed[:7], "Bearer ") {
		return strings.TrimSpace(trimmed[7:])
	}
	return trimmed
}

func headerValueString(value interface{}) (string, bool) {
	if value == nil {
		return "", false
	}
	switch v := value.(type) {
	case string:
		return v, true
	case float64, bool:
		return fmt.Sprintf("%v", v), true
	default:
		bs, err := json.Marshal(v)
		if err != nil {
			return "", false
		}
		return string(bs), true
	}
}

func setRequestHeader(req *http.Request, key, value string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	if strings.EqualFold(key, "Cookie") {
		merged := mergeCookieHeaders(req.Header.Get("Cookie"), value)
		if merged == "" {
			req.Header.Del("Cookie")
			return
		}
		req.Header.Set("Cookie", merged)
		return
	}
	req.Header.Set(key, value)
}

// applyCustomHeaders parses and applies custom headers from opt onto req.
func applyCustomHeaders(req *http.Request, opt *RequestOption) {
	if opt == nil || opt.CustomHeaders == nil || *opt.CustomHeaders == "" || *opt.CustomHeaders == "{}" {
		return
	}
	var custom map[string]interface{}
	if err := json.Unmarshal([]byte(*opt.CustomHeaders), &custom); err == nil {
		for k, raw := range custom {
			v, ok := headerValueString(raw)
			if !ok {
				continue
			}
			setRequestHeader(req, k, v)
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
	applyCustomHeaders(req, opt)

	for k, v := range headers {
		setRequestHeader(req, k, v)
	}

	// Add a default User-Agent to bypass Cloudflare tarpitting of Go-http-client.
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	}

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
	cleanToken := stripBearerPrefix(accessToken)

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

	token := extractLoginAccessToken(res)

	if token == "" {
		return &LoginResult{Success: false, Message: "No token in response"}, nil
	}

	return &LoginResult{
		Success:     true,
		AccessToken: token,
		Username:    username,
	}, nil
}

func extractLoginAccessToken(payload map[string]interface{}) string {
	candidates := []interface{}{
		payload["data"],
		payload["token"],
		payload["accessToken"],
		payload["access_token"],
	}
	if data, ok := payload["data"].(map[string]interface{}); ok {
		candidates = append(candidates, data["token"], data["accessToken"], data["access_token"])
	}
	for _, candidate := range candidates {
		if token, ok := candidate.(string); ok && strings.TrimSpace(token) != "" {
			return strings.TrimSpace(token)
		}
	}
	return ""
}

// LoginWithCookieFallback performs a username/password login and accepts either
// a JSON access token or a usable Set-Cookie session returned by the upstream.
func (b *BaseAdapter) LoginWithCookieFallback(baseURL, username, password string, opt *RequestOption) (*LoginResult, error) {
	url := fmt.Sprintf("%s/api/user/login", baseURL)
	body := map[string]string{
		"username": username,
		"password": password,
	}

	var res map[string]interface{}
	cookieResult, err := FetchJSONWithCookieRetry(url, "POST", "", map[string]string{
		"X-Requested-With": "XMLHttpRequest",
	}, body, &res, opt)
	if err != nil {
		return &LoginResult{Success: false, Message: err.Error()}, nil
	}

	success, _ := res["success"].(bool)
	message := ExtractMessage(res)
	if success {
		token := extractLoginAccessToken(res)
		hasCookie := cookieResult != nil && hasUsableSessionCookie(cookieResult.CookieHeader)

		var platformUserID int64
		if data, ok := res["data"].(map[string]interface{}); ok {
			if idFloat, ok := data["id"].(float64); ok && idFloat > 0 {
				platformUserID = int64(idFloat)
			}
		}

		// Prefer cookie when both are available: some new-api forks only accept
		// cookie/session auth on /api/user/checkin (and sign_in), while Bearer JWT
		// is only accepted on read-only endpoints like /api/user/self.
		// A cookie credential can do everything a JWT can (balance, tokens, etc.)
		// but not vice-versa; so cookie takes priority.
		if hasCookie {
			return &LoginResult{Success: true, AccessToken: cookieResult.CookieHeader, Username: username, PlatformUserID: platformUserID}, nil
		}
		if token != "" {
			return &LoginResult{Success: true, AccessToken: token, Username: username, PlatformUserID: platformUserID}, nil
		}
	}

	if message == "" {
		message = "登录失败：未获取到可用会话凭据，请改用 Cookie/Token 导入"
	}
	return &LoginResult{Success: false, Message: message}, nil
}

func hasUsableSessionCookie(cookieHeader string) bool {
	if strings.TrimSpace(cookieHeader) == "" {
		return false
	}
	ignored := map[string]bool{
		"acw_tc":     true,
		"acw_sc__v2": true,
		"cdn_sec_tc": true,
	}
	for _, pair := range strings.Split(cookieHeader, ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		eq := strings.Index(pair, "=")
		if eq <= 0 {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(pair[:eq]))
		if name == "" || ignored[name] {
			continue
		}
		if name == "session" ||
			name == "token" ||
			name == "auth_token" ||
			name == "access_token" ||
			name == "jwt" ||
			name == "jwt_token" ||
			strings.Contains(name, "session") ||
			strings.Contains(name, "token") ||
			strings.Contains(name, "auth") {
			return true
		}
	}
	return false
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

	return parseApiTokensArray(res), nil
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
