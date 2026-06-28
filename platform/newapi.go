package platform

import (
	"fmt"
	"math"
	"strings"
)

// NewApiAdapter handles new-api compatible platforms,
// implementing full cookie session retry and sign_in fallback.
type NewApiAdapter struct {
	BaseAdapter
}

func init() {
	Register(&NewApiAdapter{BaseAdapter: BaseAdapter{Name: "new-api"}})
}

func (a *NewApiAdapter) Login(baseURL, username, password string, opt *RequestOption) (*LoginResult, error) {
	return a.LoginWithCookieFallback(baseURL, username, password, opt)
}

func (a *NewApiAdapter) Checkin(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (*CheckinResult, error) {
	if !IsCookieSessionToken(accessToken) {
		headers := AuthHeaders(accessToken, platformUserID)
		url := fmt.Sprintf("%s/api/user/checkin", baseURL)
		var res map[string]interface{}
		err := a.FetchJSON(url, "POST", headers, nil, &res, opt)
		if err == nil {
			success, _ := res["success"].(bool)
			message := ExtractMessage(res)
			if success {
				if message == "" {
					message = "checkin success"
				}
				reward := ""
				if data, ok := res["data"].(map[string]interface{}); ok {
					if r, ok := data["reward"]; ok {
						reward = fmt.Sprintf("%v", r)
					}
				}
				return &CheckinResult{Success: true, Message: message, Reward: reward}, nil
			}
			if message != "" && !isMissingCheckinEndpoint(message) {
				if !isCookieSessionFailureMessage(message) {
					return &CheckinResult{Success: false, Message: message}, nil
				}
			}
		} else if err != nil && !strings.Contains(err.Error(), "HTTP 404") {
			if !isCookieSessionFailureMessage(err.Error()) {
				// pass through to cookie check
			}
		}
	}

	cookieCandidates := BuildCookieCandidates(accessToken)
	if len(cookieCandidates) == 0 {
		return &CheckinResult{Success: false, Message: "checkin failed (no cookie candidates)"}, nil
	}

	var lastErrMsg string
	for _, cookie := range cookieCandidates {
		signInURL := fmt.Sprintf("%s/api/user/sign_in", baseURL)
		var signInRes map[string]interface{}
		signInHeaders := CookieUserIDHeaders(platformUserID)

		_, err := FetchJSONWithCookieRetry(signInURL, "POST", cookie, signInHeaders, map[string]interface{}{}, &signInRes, opt)
		if err != nil {
			lastErrMsg = err.Error()
			continue
		}

		success, _ := signInRes["success"].(bool)
		msg := ExtractMessage(signInRes)
		if success {
			reward := ""
			if data, ok := signInRes["data"].(map[string]interface{}); ok {
				if r, ok := data["reward"]; ok {
					reward = fmt.Sprintf("%v", r)
				}
			}
			if msg == "" {
				msg = "checked in via sign_in"
			}
			return &CheckinResult{Success: true, Message: msg, Reward: reward}, nil
		}

		if msg != "" {
			lastErrMsg = msg
		}

		checkinURL := fmt.Sprintf("%s/api/user/checkin", baseURL)
		var checkinRes map[string]interface{}
		_, err = FetchJSONWithCookieRetry(checkinURL, "POST", cookie, signInHeaders, nil, &checkinRes, opt)
		if err != nil {
			lastErrMsg = err.Error()
			continue
		}

		checkinSuccess, _ := checkinRes["success"].(bool)
		checkinMsg := ExtractMessage(checkinRes)
		if checkinSuccess {
			if checkinMsg == "" {
				checkinMsg = "checkin success"
			}
			reward := ""
			if data, ok := checkinRes["data"].(map[string]interface{}); ok {
				if r, ok := data["reward"]; ok {
					reward = fmt.Sprintf("%v", r)
				}
			}
			return &CheckinResult{Success: true, Message: checkinMsg, Reward: reward}, nil
		}

		if checkinMsg != "" {
			lastErrMsg = checkinMsg
		}
	}

	if lastErrMsg == "" {
		lastErrMsg = "cookie checkin failed"
	}
	return &CheckinResult{Success: false, Message: lastErrMsg}, nil
}

func (a *NewApiAdapter) GetBalance(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (*BalanceInfo, error) {
	url := fmt.Sprintf("%s/api/user/self", baseURL)

	var res map[string]interface{}
	var lastErr error

	if !IsCookieSessionToken(accessToken) {
		headers := AuthHeaders(accessToken, platformUserID)
		err := a.FetchJSON(url, "GET", headers, nil, &res, opt)
		if err == nil {
			success, _ := res["success"].(bool)
			if success {
				data, _ := res["data"].(map[string]interface{})
				if data != nil {
					return parseNewApiBalance(data, 500000), nil
				}
			} else {
				lastErr = fmt.Errorf("failed: %s", ExtractMessage(res))
			}
		} else {
			lastErr = err
		}

		if lastErr != nil && !isCookieSessionFailureMessage(lastErr.Error()) {
			return nil, lastErr
		}
	}

	cookieCandidates := BuildCookieCandidates(accessToken)
	for _, cookie := range cookieCandidates {
		var cookieRes map[string]interface{}
		cookieHeaders := CookieUserIDHeaders(platformUserID)

		_, err := FetchJSONWithCookieRetry(url, "GET", cookie, cookieHeaders, nil, &cookieRes, opt)
		if err != nil {
			lastErr = err
			continue
		}

		success, _ := cookieRes["success"].(bool)
		if success {
			data, _ := cookieRes["data"].(map[string]interface{})
			if data != nil {
				return parseNewApiBalance(data, 500000), nil
			}
		} else {
			lastErr = fmt.Errorf("cookie failed: %s", ExtractMessage(cookieRes))
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("failed to fetch balance: %w", lastErr)
	}
	return nil, fmt.Errorf("failed to fetch balance: no valid method found")
}

func (a *NewApiAdapter) GetApiTokens(baseURL, accessToken string, platformUserID int64, opt *RequestOption) ([]ApiTokenInfo, error) {
	url := fmt.Sprintf("%s/api/token/?p=0&size=100", baseURL)
	var res map[string]interface{}
	var lastErr error

	if !IsCookieSessionToken(accessToken) {
		headers := AuthHeaders(accessToken, platformUserID)
		err := a.FetchJSON(url, "GET", headers, nil, &res, opt)
		if err == nil {
			success, _ := res["success"].(bool)
			if success {
				return parseApiTokensArray(res), nil
			}
			lastErr = fmt.Errorf("failed: %s", ExtractMessage(res))
		} else {
			lastErr = err
		}
		if lastErr != nil && !isCookieSessionFailureMessage(lastErr.Error()) {
			return nil, lastErr
		}
	}

	cookieCandidates := BuildCookieCandidates(accessToken)
	for _, cookie := range cookieCandidates {
		var cookieRes map[string]interface{}
		cookieHeaders := CookieUserIDHeaders(platformUserID)

		_, err := FetchJSONWithCookieRetry(url, "GET", cookie, cookieHeaders, nil, &cookieRes, opt)
		if err != nil {
			lastErr = err
			continue
		}
		success, _ := cookieRes["success"].(bool)
		if success {
			return parseApiTokensArray(cookieRes), nil
		}
		lastErr = fmt.Errorf("cookie failed: %s", ExtractMessage(cookieRes))
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("failed to fetch api tokens")
}

func (a *NewApiAdapter) GetModels(baseURL, accessToken string, platformUserID int64, opt *RequestOption) ([]string, error) {
	url := fmt.Sprintf("%s/v1/models", baseURL)
	var res map[string]interface{}
	var lastErr error

	if !IsCookieSessionToken(accessToken) {
		headers := AuthHeaders(accessToken, platformUserID)
		err := a.FetchJSON(url, "GET", headers, nil, &res, opt)
		if err == nil {
			if data, ok := res["data"].([]interface{}); ok {
				return parseModelsArray(data), nil
			}
			lastErr = fmt.Errorf("no data in models response")
		} else {
			lastErr = err
		}
		if lastErr != nil && !isCookieSessionFailureMessage(lastErr.Error()) {
			return nil, lastErr
		}
	}

	cookieCandidates := BuildCookieCandidates(accessToken)
	for _, cookie := range cookieCandidates {
		var cookieRes map[string]interface{}
		cookieHeaders := CookieUserIDHeaders(platformUserID)

		_, err := FetchJSONWithCookieRetry(url, "GET", cookie, cookieHeaders, nil, &cookieRes, opt)
		if err != nil {
			lastErr = err
			continue
		}
		if data, ok := cookieRes["data"].([]interface{}); ok {
			return parseModelsArray(data), nil
		}
		lastErr = fmt.Errorf("cookie failed: no data in response")
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("failed to fetch models")
}

func (a *NewApiAdapter) VerifyToken(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (*VerifyTokenResult, error) {
	userURL := fmt.Sprintf("%s/api/user/self", baseURL)
	var lastErr error

	// Try session validation via Bearer
	if !IsCookieSessionToken(accessToken) {
		headers := AuthHeaders(accessToken, platformUserID)
		var userRes map[string]interface{}
		err := a.FetchJSON(userURL, "GET", headers, nil, &userRes, opt)
		if err == nil {
			success, _ := userRes["success"].(bool)
			if success {
				ui, bi := parseUserInfoAndBalance(userRes["data"])
				apiToken, _ := a.GetApiToken(baseURL, accessToken, platformUserID, opt)
				return &VerifyTokenResult{
					TokenType: "session",
					UserInfo:  ui,
					Balance:   bi,
					ApiToken:  apiToken,
				}, nil
			}
			lastErr = fmt.Errorf("failed: %s", ExtractMessage(userRes))
		} else {
			lastErr = err
		}
	}

	// Try session validation via Cookie
	cookieCandidates := BuildCookieCandidates(accessToken)
	for _, cookie := range cookieCandidates {
		var userRes map[string]interface{}
		cookieHeaders := CookieUserIDHeaders(platformUserID)
		_, err := FetchJSONWithCookieRetry(userURL, "GET", cookie, cookieHeaders, nil, &userRes, opt)
		if err == nil {
			success, _ := userRes["success"].(bool)
			if success {
				ui, bi := parseUserInfoAndBalance(userRes["data"])
				apiToken, _ := a.GetApiToken(baseURL, cookie, platformUserID, opt)
				return &VerifyTokenResult{
					TokenType: "session",
					UserInfo:  ui,
					Balance:   bi,
					ApiToken:  apiToken,
				}, nil
			}
			lastErr = fmt.Errorf("failed: %s", ExtractMessage(userRes))
		} else {
			lastErr = err
		}
	}

	// Try apikey validation (mostly Bearer)
	models, err := a.GetModels(baseURL, accessToken, platformUserID, opt)
	if err == nil && len(models) > 0 {
		return &VerifyTokenResult{
			TokenType: "apikey",
			Models:    models,
		}, nil
	}

	if lastErr != nil {
		// pass through
	}

	return &VerifyTokenResult{
		TokenType: "unknown",
	}, nil
}

// Helpers

func tokenItemsFromPayload(payload interface{}) []interface{} {
	if arr, ok := payload.([]interface{}); ok {
		return arr
	}
	m, ok := payload.(map[string]interface{})
	if !ok {
		return nil
	}
	for _, key := range []string{"data", "items", "list"} {
		if arr, ok := m[key].([]interface{}); ok {
			return arr
		}
	}
	if dataMap, ok := m["data"].(map[string]interface{}); ok {
		for _, key := range []string{"items", "data", "list"} {
			if arr, ok := dataMap[key].([]interface{}); ok {
				return arr
			}
		}
	}
	return nil
}

func parseApiTokensArray(payload interface{}) []ApiTokenInfo {
	arr := tokenItemsFromPayload(payload)
	var tokens []ApiTokenInfo
	for i, item := range arr {
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
				if i == 0 {
					name = "default"
				} else {
					name = fmt.Sprintf("token-%d", i+1)
				}
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
	return tokens
}

func parseModelsArray(data []interface{}) []string {
	var models []string
	for _, item := range data {
		if m, ok := item.(map[string]interface{}); ok {
			if id, ok := m["id"].(string); ok && id != "" {
				models = append(models, id)
			}
		}
	}
	return models
}

func parseUserInfoAndBalance(dataObj interface{}) (*UserInfo, *BalanceInfo) {
	data, ok := dataObj.(map[string]interface{})
	if !ok || data == nil {
		return nil, nil
	}
	ui := &UserInfo{
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
	bi := &BalanceInfo{
		Balance: quota,
		Used:    used,
		Quota:   quota + used,
	}
	return ui, bi
}

func parseNewApiBalance(data map[string]interface{}, divisor float64) *BalanceInfo {
	quota := toFloat(data["quota"]) / divisor
	used := toFloat(data["used_quota"]) / divisor
	return &BalanceInfo{
		Balance: quota,
		Used:    used,
		Quota:   quota + used,
	}
}

func isMissingCheckinEndpoint(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "invalid url") ||
		strings.Contains(lower, "404") ||
		strings.Contains(lower, "not found") ||
		strings.Contains(lower, "not support") ||
		strings.Contains(lower, "does not support checkin")
}

func isCookieSessionFailureMessage(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "access token") ||
		strings.Contains(lower, "unauthorized") ||
		strings.Contains(lower, "forbidden") ||
		strings.Contains(lower, "new-api-user") ||
		strings.Contains(lower, "user id") ||
		strings.Contains(lower, "invalid token") ||
		strings.Contains(lower, "expired") ||
		strings.Contains(lower, "无权") ||
		strings.Contains(lower, "未登录") ||
		strings.Contains(lower, "未提供") ||
		strings.Contains(lower, "未授权") ||
		strings.Contains(lower, "not login") ||
		strings.Contains(lower, "not logged") ||
		strings.Contains(lower, "invalid url (post /api/user/checkin)") ||
		strings.Contains(lower, "http 404")
}

func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case float64:
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return 0
		}
		return n
	case int64:
		return float64(n)
	case int:
		return float64(n)
	default:
		return 0
	}
}
