package platform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	Balance  float64  `json:"balance"`
	Used     float64  `json:"used"`
	Quota    float64  `json:"quota"`
}

// LoginResult is returned from Login calls.
type LoginResult struct {
	Success     bool   `json:"success"`
	AccessToken string `json:"access_token,omitempty"`
	Username    string `json:"username,omitempty"`
	Message     string `json:"message,omitempty"`
}

// Adapter is the interface each platform must implement.
type Adapter interface {
	PlatformName() string
	Checkin(baseURL, accessToken string, platformUserID int64) (*CheckinResult, error)
	GetBalance(baseURL, accessToken string, platformUserID int64) (*BalanceInfo, error)
}

// BaseAdapter provides shared HTTP helpers.
type BaseAdapter struct {
	Name string
}

func (b *BaseAdapter) PlatformName() string {
	return b.Name
}

// FetchJSON makes a JSON request and decodes response.
func (b *BaseAdapter) FetchJSON(url, method string, headers map[string]string, body interface{}, out interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		bs, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(bs)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 30 * time.Second}
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
