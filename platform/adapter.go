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
}

// BaseAdapter provides shared HTTP helpers.
type BaseAdapter struct {
	Name string
}

func (b *BaseAdapter) PlatformName() string {
	return b.Name
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
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment, // default
	}

	if opt != nil {
		if opt.CustomHeaders != nil && *opt.CustomHeaders != "" {
			var custom map[string]string
			if err := json.Unmarshal([]byte(*opt.CustomHeaders), &custom); err == nil {
				for k, v := range custom {
					req.Header.Set(k, v)
				}
			}
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
	}

	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
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
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	if out != nil {
		return json.Unmarshal(respBody, out)
	}
	return nil
}

// AuthHeaders builds common user-id headers used by new-api / one-api family.
func AuthHeaders(accessToken string, platformUserID int64) map[string]string {
	h := map[string]string{
		"Authorization": "Bearer " + accessToken,
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
	url := fmt.Sprintf("%s/api/token/?p=0&size=10", baseURL)
	headers := AuthHeaders(accessToken, platformUserID)

	var res map[string]interface{}
	err := b.FetchJSON(url, "GET", headers, nil, &res, opt)
	if err != nil {
		return "", err
	}

	success, _ := res["success"].(bool)
	if !success {
		return "", fmt.Errorf("fetch token failed: %s", ExtractMessage(res))
	}

	data, _ := res["data"].([]interface{})
	for _, item := range data {
		if m, ok := item.(map[string]interface{}); ok {
			// Check if enabled or status is 1
			enabled := false
			if status, ok := m["status"].(float64); ok && status == 1 {
				enabled = true
			} else if e, ok := m["enabled"].(bool); ok && e {
				enabled = true
			}
			// In some platforms, enabled is a boolean, some status is int
			
			// Let's just find the first token if we cannot determine status, or if it's enabled
			// NewAPI uses status=1, OneAPI uses status=1
			if !enabled {
			    // fallback if no explicit enabled/status found? We prefer status=1
			    if _, hasStatus := m["status"]; !hasStatus {
			        if _, hasEnabled := m["enabled"]; !hasEnabled {
			            enabled = true // default to true if we can't parse
			        }
			    }
			}
			
			if enabled {
				if key, ok := m["key"].(string); ok && key != "" {
					return key, nil
				}
			}
		}
	}
	
	// fallback: just return the first one if we have any but none are clearly enabled
	if len(data) > 0 {
	    if m, ok := data[0].(map[string]interface{}); ok {
	        if key, ok := m["key"].(string); ok && key != "" {
	            return key, nil
	        }
	    }
	}

	return "", fmt.Errorf("no valid api token found")
}
