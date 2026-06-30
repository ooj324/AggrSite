package platform

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// AgentRouterAdapter is a New API fork whose daily check-in is performed by
// the login flow. It still uses New-API-User plus a browser session cookie for
// console APIs such as /api/user/self and /api/log/self.
type AgentRouterAdapter struct {
	NewApiAdapter
}

func init() {
	Register(&AgentRouterAdapter{NewApiAdapter: NewApiAdapter{BaseAdapter: BaseAdapter{Name: "agentrouter"}}})
}

func decodeBase64Any(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(raw); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.URLEncoding.DecodeString(raw); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(raw); err == nil {
		return decoded, nil
	}
	return nil, fmt.Errorf("invalid base64")
}

func int64FromSessionValue(value interface{}) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int8:
		return int64(v)
	case int16:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case uint:
		return int64(v)
	case uint8:
		return int64(v)
	case uint16:
		return int64(v)
	case uint32:
		return int64(v)
	case uint64:
		if v <= uint64(^uint64(0)>>1) {
			return int64(v)
		}
	case float64:
		return int64(v)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return n
	}
	return 0
}

func sessionMapUserID(data map[interface{}]interface{}) int64 {
	for key, value := range data {
		name := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", key)))
		switch name {
		case "id", "user_id", "userid", "user-id", "uid":
			if id := int64FromSessionValue(value); id > 0 {
				return id
			}
		}
	}
	return 0
}

func sessionStringMapUserID(data map[string]interface{}) int64 {
	for key, value := range data {
		name := strings.ToLower(strings.TrimSpace(key))
		switch name {
		case "id", "user_id", "userid", "user-id", "uid":
			if id := int64FromSessionValue(value); id > 0 {
				return id
			}
		}
	}
	return 0
}

func tryDecodeAgentRouterSessionUserID(accessToken string) int64 {
	raw := strings.TrimSpace(stripBearerPrefix(accessToken))
	if raw == "" {
		return 0
	}
	if value, ok := CookieValueFromHeader(raw, "session"); ok {
		raw = value
	}

	decoded, err := decodeBase64Any(raw)
	if err != nil {
		return 0
	}
	parts := bytes.SplitN(decoded, []byte("|"), 3)
	if len(parts) < 3 {
		return 0
	}
	payload, err := decodeBase64Any(string(parts[1]))
	if err != nil {
		return 0
	}

	var ifaceMap map[interface{}]interface{}
	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&ifaceMap); err == nil {
		if id := sessionMapUserID(ifaceMap); id > 0 {
			return id
		}
	}

	var stringMap map[string]interface{}
	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&stringMap); err == nil {
		if id := sessionStringMapUserID(stringMap); id > 0 {
			return id
		}
	}
	return 0
}

func looksLikeAgentRouterSession(accessToken string) bool {
	raw := strings.TrimSpace(stripBearerPrefix(accessToken))
	if raw == "" {
		return false
	}
	if strings.HasPrefix(raw, "sk-") {
		return false
	}
	return IsCookieSessionToken(raw)
}

func agentRouterBrowserHeaders(baseURL string, platformUserID int64) map[string]string {
	headers := CookieUserIDHeaders(platformUserID)
	if headers == nil {
		headers = map[string]string{}
	}
	headers["Accept"] = "application/json, text/plain, */*"
	headers["Accept-Language"] = "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6"
	headers["Cache-Control"] = "no-store"
	headers["Connection"] = "keep-alive"
	headers["Sec-Fetch-Dest"] = "empty"
	headers["Sec-Fetch-Mode"] = "cors"
	headers["Sec-Fetch-Site"] = "same-origin"
	headers["sec-ch-ua"] = `"Microsoft Edge";v="149", "Chromium";v="149", "Not)A;Brand";v="24"`
	headers["sec-ch-ua-mobile"] = "?0"
	headers["sec-ch-ua-platform"] = `"Windows"`
	origin := strings.TrimRight(baseURL, "/")
	if origin != "" {
		headers["Referer"] = origin + "/console"
	}
	return headers
}

func (a *AgentRouterAdapter) resolveAgentRouterUserID(baseURL, accessToken string, platformUserID int64, opt *RequestOption) int64 {
	if platformUserID > 0 {
		return platformUserID
	}
	if id := tryDecodeAgentRouterSessionUserID(accessToken); id > 0 {
		return id
	}
	return a.NewApiAdapter.discoverUserId(baseURL, accessToken, 0, opt)
}

func (a *AgentRouterAdapter) requireAgentRouterUserID(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (int64, error) {
	userID := a.resolveAgentRouterUserID(baseURL, accessToken, platformUserID, opt)
	if userID <= 0 && looksLikeAgentRouterSession(accessToken) {
		return 0, fmt.Errorf("New-API-User user id required for AgentRouter session cookie")
	}
	return userID, nil
}

func (a *AgentRouterAdapter) Login(baseURL, username, password string, opt *RequestOption) (*LoginResult, error) {
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
	if !success {
		if message == "" {
			message = "Login failed"
		}
		return &LoginResult{Success: false, Message: message}, nil
	}

	token := extractLoginAccessToken(res)
	hasCookie := cookieResult != nil && hasUsableSessionCookie(cookieResult.CookieHeader)

	data, _ := res["data"].(map[string]interface{})
	var platformUserID int64
	if data != nil {
		platformUserID = int64FromSessionValue(data["id"])
	}
	if platformUserID <= 0 && cookieResult != nil {
		platformUserID = tryDecodeAgentRouterSessionUserID(cookieResult.CookieHeader)
	}

	if checkedIn, _ := data["checked_in"].(bool); checkedIn {
		message = "每日签到成功，新增额度已到账"
	} else if message == "" {
		message = "登录成功"
	}

	if hasCookie {
		return &LoginResult{Success: true, AccessToken: cookieResult.CookieHeader, Username: username, PlatformUserID: platformUserID, Message: message}, nil
	}
	if token != "" {
		return &LoginResult{Success: true, AccessToken: token, Username: username, PlatformUserID: platformUserID, Message: message}, nil
	}

	return &LoginResult{Success: false, Message: "登录失败：未获取到可用会话凭据，请改用 Cookie/Token 导入"}, nil
}

func (a *AgentRouterAdapter) GetApiToken(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (string, error) {
	tokens, err := a.GetApiTokens(baseURL, accessToken, platformUserID, opt)
	if err != nil {
		return "", err
	}
	for _, token := range tokens {
		if token.Enabled && strings.TrimSpace(token.Key) != "" {
			return strings.TrimSpace(token.Key), nil
		}
	}
	if len(tokens) > 0 && strings.TrimSpace(tokens[0].Key) != "" {
		return strings.TrimSpace(tokens[0].Key), nil
	}
	return "", fmt.Errorf("no valid api token found")
}

func (a *AgentRouterAdapter) GetApiTokens(baseURL, accessToken string, platformUserID int64, opt *RequestOption) ([]ApiTokenInfo, error) {
	userID, err := a.requireAgentRouterUserID(baseURL, accessToken, platformUserID, opt)
	if err != nil {
		return nil, err
	}

	if looksLikeAgentRouterSession(accessToken) {
		url := fmt.Sprintf("%s/api/token/?p=0&size=100", baseURL)
		var lastErr error
		for _, cookie := range BuildCookieCandidates(accessToken) {
			var res map[string]interface{}
			_, err := FetchJSONWithCookieRetry(url, "GET", cookie, agentRouterBrowserHeaders(baseURL, userID), nil, &res, opt)
			if err != nil {
				lastErr = err
				continue
			}
			success, _ := res["success"].(bool)
			if !success {
				lastErr = fmt.Errorf("fetch token failed: %s", ExtractMessage(res))
				continue
			}
			return parseApiTokensArray(res), nil
		}
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("failed to fetch api tokens")
	}

	return a.NewApiAdapter.GetApiTokens(baseURL, accessToken, userID, opt)
}

func (a *AgentRouterAdapter) GetBalance(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (*BalanceInfo, error) {
	userID, err := a.requireAgentRouterUserID(baseURL, accessToken, platformUserID, opt)
	if err != nil {
		return nil, err
	}

	if looksLikeAgentRouterSession(accessToken) {
		url := fmt.Sprintf("%s/api/user/self", baseURL)
		var lastErr error
		for _, cookie := range BuildCookieCandidates(accessToken) {
			var res map[string]interface{}
			_, err := FetchJSONWithCookieRetry(url, "GET", cookie, agentRouterBrowserHeaders(baseURL, userID), nil, &res, opt)
			if err != nil {
				lastErr = err
				continue
			}
			success, _ := res["success"].(bool)
			if !success {
				lastErr = fmt.Errorf("failed: %s", ExtractMessage(res))
				continue
			}
			data, _ := res["data"].(map[string]interface{})
			if data == nil {
				return nil, fmt.Errorf("no data in balance response")
			}
			return parseNewApiBalance(data, 500000), nil
		}
		if lastErr != nil {
			return nil, fmt.Errorf("failed to fetch balance: %w", lastErr)
		}
		return nil, fmt.Errorf("failed to fetch balance")
	}

	return a.NewApiAdapter.GetBalance(baseURL, accessToken, userID, opt)
}

func (a *AgentRouterAdapter) GetModels(baseURL, accessToken string, platformUserID int64, opt *RequestOption) ([]string, error) {
	return a.NewApiAdapter.GetModels(baseURL, accessToken, platformUserID, opt)
}

func (a *AgentRouterAdapter) probeSessionViaLog(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (*UserInfo, string, error) {
	now := time.Now()
	start := now.AddDate(0, 0, -1).Unix()
	end := now.Add(time.Hour).Unix()
	logURL := fmt.Sprintf("%s/api/log/self?p=1&page_size=1&type=0&token_name=&model_name=&start_timestamp=%d&end_timestamp=%d&group=", baseURL, start, end)

	var lastErr error
	for _, cookie := range BuildCookieCandidates(accessToken) {
		var res map[string]interface{}
		_, err := FetchJSONWithCookieRetry(logURL, "GET", cookie, agentRouterBrowserHeaders(baseURL, platformUserID), nil, &res, opt)
		if err != nil {
			lastErr = err
			continue
		}
		success, _ := res["success"].(bool)
		if !success {
			lastErr = fmt.Errorf("log probe failed: %s", ExtractMessage(res))
			continue
		}

		ui := &UserInfo{}
		if data, _ := res["data"].(map[string]interface{}); data != nil {
			if items, _ := data["items"].([]interface{}); len(items) > 0 {
				if row, _ := items[0].(map[string]interface{}); row != nil {
					if username, _ := row["username"].(string); strings.TrimSpace(username) != "" {
						ui.Username = strings.TrimSpace(username)
					}
				}
			}
		}
		if ui.Username == "" {
			ui = nil
		}
		return ui, "session verified via /api/log/self", nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("log probe failed: no cookie candidate")
	}
	return nil, "", lastErr
}

func (a *AgentRouterAdapter) VerifyToken(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (*VerifyTokenResult, error) {
	userID, err := a.requireAgentRouterUserID(baseURL, accessToken, platformUserID, opt)
	if err != nil {
		return nil, err
	}

	userURL := fmt.Sprintf("%s/api/user/self", baseURL)
	var lastErr error

	if looksLikeAgentRouterSession(accessToken) {
		for _, cookie := range BuildCookieCandidates(accessToken) {
			var userRes map[string]interface{}
			_, err := FetchJSONWithCookieRetry(userURL, "GET", cookie, agentRouterBrowserHeaders(baseURL, userID), nil, &userRes, opt)
			if err != nil {
				lastErr = err
				continue
			}
			success, _ := userRes["success"].(bool)
			if success {
				ui, bi := parseUserInfoAndBalance(userRes["data"])
				apiToken, _ := a.GetApiToken(baseURL, cookie, userID, opt)
				return &VerifyTokenResult{
					TokenType: "session",
					UserInfo:  ui,
					Balance:   bi,
					ApiToken:  apiToken,
				}, nil
			}
			lastErr = fmt.Errorf("failed: %s", ExtractMessage(userRes))
		}
		if ui, message, err := a.probeSessionViaLog(baseURL, accessToken, userID, opt); err == nil {
			apiToken, _ := a.GetApiToken(baseURL, accessToken, userID, opt)
			return &VerifyTokenResult{
				TokenType: "session",
				Message:   message,
				UserInfo:  ui,
				ApiToken:  apiToken,
			}, nil
		} else if lastErr == nil {
			lastErr = err
		}
		message := "session cookie not accepted"
		if lastErr != nil {
			message = lastErr.Error()
		}
		return &VerifyTokenResult{TokenType: "unknown", Message: message}, nil
	}

	var userRes map[string]interface{}
	err = a.FetchJSON(userURL, "GET", AuthHeaders(accessToken, userID), nil, &userRes, opt)
	if err == nil {
		success, _ := userRes["success"].(bool)
		if success {
			ui, bi := parseUserInfoAndBalance(userRes["data"])
			apiToken, _ := a.GetApiToken(baseURL, accessToken, userID, opt)
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

	models, err := a.GetModels(baseURL, accessToken, userID, opt)
	if err == nil && len(models) > 0 {
		return &VerifyTokenResult{TokenType: "apikey", Models: models}, nil
	}
	if lastErr != nil {
		return &VerifyTokenResult{TokenType: "unknown"}, nil
	}
	return &VerifyTokenResult{TokenType: "unknown"}, nil
}

func extractAgentRouterCheckinLog(res map[string]interface{}) (bool, string) {
	data, _ := res["data"].(map[string]interface{})
	if data == nil {
		return false, ""
	}
	items, _ := data["items"].([]interface{})
	for _, item := range items {
		row, _ := item.(map[string]interface{})
		if row == nil {
			continue
		}
		content := strings.TrimSpace(fmt.Sprintf("%v", row["content"]))
		if content == "" {
			continue
		}
		if int(toFloat(row["type"])) == 4 && strings.Contains(content, "签到") {
			return true, content
		}
		if strings.Contains(content, "每日签到成功") {
			return true, content
		}
	}
	return false, ""
}

func (a *AgentRouterAdapter) findTodayCheckinLog(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (bool, string, error) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	end := now.Add(time.Hour).Unix()
	logURL := fmt.Sprintf("%s/api/log/self?p=1&page_size=20&type=0&token_name=&model_name=&start_timestamp=%d&end_timestamp=%d&group=", baseURL, start, end)

	var lastErr error
	for _, cookie := range BuildCookieCandidates(accessToken) {
		var res map[string]interface{}
		_, err := FetchJSONWithCookieRetry(logURL, "GET", cookie, agentRouterBrowserHeaders(baseURL, platformUserID), nil, &res, opt)
		if err != nil {
			lastErr = err
			continue
		}
		if ok, msg := extractAgentRouterCheckinLog(res); ok {
			return true, msg, nil
		}
		return false, "", nil
	}
	if lastErr != nil {
		return false, "", lastErr
	}
	return false, "", fmt.Errorf("no cookie candidate")
}

func (a *AgentRouterAdapter) Checkin(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (*CheckinResult, error) {
	userID, err := a.requireAgentRouterUserID(baseURL, accessToken, platformUserID, opt)
	if err != nil {
		return &CheckinResult{Success: false, Message: err.Error()}, nil
	}
	if ok, msg, err := a.findTodayCheckinLog(baseURL, accessToken, userID, opt); err == nil && ok {
		if msg == "" {
			msg = "今日已签到"
		}
		return &CheckinResult{Success: true, Message: msg}, nil
	}
	return &CheckinResult{Success: false, Message: "AgentRouter does not support standalone checkin endpoint; daily check-in is triggered by login"}, nil
}
