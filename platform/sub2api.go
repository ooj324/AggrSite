package platform

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	sub2APIRateLimitBackoff = time.Minute
	sub2APIFallbackTokenTTL = 15 * time.Minute
)

// Sub2ApiAdapter: JWT auth, no login/checkin; balance from /api/v1/auth/me.
type Sub2ApiAdapter struct {
	BaseAdapter
}

func init() {
	Register(&Sub2ApiAdapter{BaseAdapter: BaseAdapter{Name: "sub2api"}})
}

func (a *Sub2ApiAdapter) Checkin(_ string, _ string, _ int64, _ *RequestOption) (*CheckinResult, error) {
	return &CheckinResult{Success: false, Message: "Check-in is not supported by Sub2API"}, nil
}

func normalizeSub2BaseURL(baseURL string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/")
}

func sub2AuthHeaders(accessToken string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + stripBearerPrefix(accessToken)}
}

func parseSub2Code(raw interface{}) (int, bool) {
	switch v := raw.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case int64:
		return int(v), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(v))
		return parsed, err == nil
	default:
		return 0, false
	}
}

func parseSub2Envelope(body map[string]interface{}, endpoint string) (interface{}, error) {
	code, ok := parseSub2Code(body["code"])
	if !ok {
		return nil, fmt.Errorf("invalid response format from %s", endpoint)
	}
	if code != 0 {
		msg := ExtractMessage(body)
		if msg == "" {
			msg = fmt.Sprintf("Error code %d from %s", code, endpoint)
		}
		return nil, fmt.Errorf("%s", msg)
	}
	data, ok := body["data"]
	if !ok {
		return nil, fmt.Errorf("missing data in response from %s", endpoint)
	}
	return data, nil
}

func fetchSub2AuthMe(a *Sub2ApiAdapter, baseURL, accessToken string, opt *RequestOption) (map[string]interface{}, error) {
	base := normalizeSub2BaseURL(baseURL)
	endpoint := "/api/v1/auth/me"
	var res map[string]interface{}
	if err := a.FetchJSON(base+endpoint, "GET", sub2AuthHeaders(accessToken), nil, &res, opt); err != nil {
		return nil, fmt.Errorf("failed to fetch auth/me: %w", err)
	}
	data, err := parseSub2Envelope(res, endpoint)
	if err != nil {
		return nil, err
	}
	user, _ := data.(map[string]interface{})
	if user == nil {
		return nil, fmt.Errorf("no data in auth/me response")
	}
	return user, nil
}

func (a *Sub2ApiAdapter) GetBalance(baseURL, accessToken string, _ int64, opt *RequestOption) (*BalanceInfo, error) {
	data, err := fetchSub2AuthMe(a, baseURL, accessToken, opt)
	if err != nil {
		return nil, err
	}

	balance := toFloat(data["balance"])
	return &BalanceInfo{
		Balance: balance,
		Used:    0,
		Quota:   balance,
	}, nil
}

func (a *Sub2ApiAdapter) Login(_ string, _ string, _ string, _ *RequestOption) (*LoginResult, error) {
	return &LoginResult{Success: false, Message: "Sub2API uses JWT authentication; login is not supported"}, nil
}

func sub2RefreshReason(body map[string]interface{}) string {
	if body == nil {
		return ""
	}
	if reason, ok := body["reason"].(string); ok {
		return strings.ToUpper(strings.TrimSpace(reason))
	}
	if errObj, ok := body["error"].(map[string]interface{}); ok {
		for _, key := range []string{"reason", "code"} {
			if reason, ok := errObj[key].(string); ok {
				return strings.ToUpper(strings.TrimSpace(reason))
			}
		}
	}
	return ""
}

func sub2RefreshReasonIsFatal(reason string) bool {
	switch strings.ToUpper(strings.TrimSpace(reason)) {
	case "REFRESH_TOKEN_INVALID", "REFRESH_TOKEN_EXPIRED", "REFRESH_TOKEN_REUSED", "TOKEN_REVOKED", "SESSION_BINDING_MISMATCH":
		return true
	}
	return false
}

func sub2RefreshFailure(err error, body map[string]interface{}) *RefreshResult {
	message := ExtractMessage(body)
	if message == "" && err != nil {
		message = err.Error()
	}
	if message == "" {
		message = "Sub2API refresh failed"
	}

	reason := sub2RefreshReason(body)
	if reason != "" && !strings.Contains(strings.ToUpper(message), reason) {
		message += " (" + reason + ")"
	}
	result := &RefreshResult{Success: false, Message: message}

	status := HTTPStatusFromError(err)
	if status == 0 {
		if code, ok := parseSub2Code(body["code"]); ok {
			status = code
		}
	}

	if status == http.StatusTooManyRequests {
		result.RetryAfter = RetryAfterFromError(err)
		if result.RetryAfter <= 0 {
			result.RetryAfter = sub2APIRateLimitBackoff
		}
		return result
	}
	if sub2RefreshReasonIsFatal(reason) {
		result.CredentialDead = true
		return result
	}

	// Except for middleware rate limiting, a failed one-time exchange has an
	// ambiguous outcome: the server may have consumed the old token before the
	// response was lost or a later step failed. Automatic retry would submit a
	// known-possibly-used token, so require a fresh login instead.
	result.CredentialDead = true
	if !strings.Contains(strings.ToLower(result.Message), "re-login") {
		result.Message += "; automatic retry disabled because Sub2API refresh tokens are one-time credentials; re-login required"
	}
	return result
}

func (a *Sub2ApiAdapter) RefreshAuth(baseURL, _ string, extraConfig string, opt *RequestOption) (*RefreshResult, error) {
	cfg := map[string]interface{}{}
	if strings.TrimSpace(extraConfig) != "" {
		if err := json.Unmarshal([]byte(extraConfig), &cfg); err != nil {
			return &RefreshResult{Success: false, Message: "Invalid extraConfig format", CredentialDead: true}, nil
		}
		if cfg == nil {
			cfg = map[string]interface{}{}
		}
	}

	sub2apiAuth, ok := cfg[Sub2APIAuthConfigKey].(map[string]interface{})
	if !ok || sub2apiAuth == nil {
		return &RefreshResult{Success: false, Message: "No sub2apiAuth found in extraConfig", CredentialDead: true}, nil
	}

	refreshToken, _ := sub2apiAuth[RefreshTokenKey].(string)
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return &RefreshResult{Success: false, Message: "No refreshToken found in sub2apiAuth", CredentialDead: true}, nil
	}

	base := normalizeSub2BaseURL(baseURL)
	url := fmt.Sprintf("%s/api/v1/auth/refresh", base)

	body := map[string]string{
		"refresh_token": refreshToken,
	}

	// Refresh tokens are one-time credentials upstream. Never retry this request
	// with/without Authorization: if the first response was lost after the server
	// rotated the token, submitting the old token again can only destroy recovery.
	var res map[string]interface{}
	_, err := fetchJSONOnce(url, "POST", "", nil, body, &res, opt)
	if err != nil {
		return sub2RefreshFailure(err, res), nil
	}

	dataRaw, err := parseSub2Envelope(res, "/api/v1/auth/refresh")
	if err != nil {
		return sub2RefreshFailure(err, res), nil
	}
	data, _ := dataRaw.(map[string]interface{})
	if data == nil {
		return &RefreshResult{
			Success:        false,
			Message:        "Sub2API accepted the refresh token but returned no token data; re-login required",
			CredentialDead: true,
		}, nil
	}

	newAccessToken, _ := data["access_token"].(string)
	newRefreshToken, _ := data["refresh_token"].(string)
	expiresIn, _ := data["expires_in"].(float64)
	newAccessToken = strings.TrimSpace(newAccessToken)
	newRefreshToken = strings.TrimSpace(newRefreshToken)

	if newAccessToken == "" || newRefreshToken == "" {
		return &RefreshResult{
			Success:        false,
			Message:        "Sub2API rotated the session but returned incomplete token data; re-login required",
			CredentialDead: true,
		}, nil
	}
	if newRefreshToken == refreshToken {
		return &RefreshResult{
			Success:        false,
			Message:        "Sub2API refresh response did not rotate the one-time refresh token; re-login required",
			CredentialDead: true,
		}, nil
	}

	tokenExpiresAt := int64(0)
	if expiresIn > 0 {
		tokenExpiresAt = time.Now().UnixMilli() + int64(expiresIn*1000)
	}
	if tokenExpiresAt <= 0 {
		tokenExpiresAt = JwtExpiresAtMillis(newAccessToken)
	}
	if tokenExpiresAt <= 0 {
		tokenExpiresAt = time.Now().Add(sub2APIFallbackTokenTTL).UnixMilli()
	}

	sub2apiAuth[RefreshTokenKey] = newRefreshToken
	cfg[Sub2APIAuthConfigKey] = sub2apiAuth
	SetManagedTokenExpiresAt(cfg, tokenExpiresAt)

	newExtraConfigBytes, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal new extraConfig: %w", err)
	}

	return &RefreshResult{
		Success:     true,
		AccessToken: newAccessToken,
		ExtraConfig: string(newExtraConfigBytes),
		Message:     "Refreshed",
	}, nil
}

func parseSub2TokenEnabled(raw interface{}) bool {
	switch v := raw.(type) {
	case bool:
		return v
	case float64:
		return v == 1
	case string:
		normalized := strings.ToLower(strings.TrimSpace(v))
		if normalized == "" {
			return true
		}
		if normalized == "inactive" || normalized == "disabled" || normalized == "false" || normalized == "0" || normalized == "off" {
			return false
		}
		return true
	default:
		return true
	}
}

func parseSub2TokenItems(payload interface{}) []ApiTokenInfo {
	arr := tokenItemsFromPayload(payload)
	tokens := make([]ApiTokenInfo, 0, len(arr))
	for i, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		key, _ := m["key"].(string)
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		name, _ := m["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			if id := strings.TrimSpace(fmt.Sprintf("%v", m["id"])); id != "" && id != "<nil>" {
				name = "token-" + id
			} else if i == 0 {
				name = "default"
			} else {
				name = fmt.Sprintf("token-%d", i+1)
			}
		}
		group := ""
		if groupID := strings.TrimSpace(fmt.Sprintf("%v", m["group_id"])); groupID != "" && groupID != "<nil>" {
			group = groupID
		} else if groupID := strings.TrimSpace(fmt.Sprintf("%v", m["groupId"])); groupID != "" && groupID != "<nil>" {
			group = groupID
		} else if groupName, ok := m["group_name"].(string); ok {
			group = strings.TrimSpace(groupName)
		} else if groupName, ok := m["group"].(string); ok {
			group = strings.TrimSpace(groupName)
		}
		tokens = append(tokens, ApiTokenInfo{
			Name:       name,
			Key:        key,
			Enabled:    parseSub2TokenEnabled(m["status"]),
			TokenGroup: group,
		})
	}
	return tokens
}

func (a *Sub2ApiAdapter) GetApiTokens(baseURL, accessToken string, _ int64, opt *RequestOption) ([]ApiTokenInfo, error) {
	base := normalizeSub2BaseURL(baseURL)
	for _, endpoint := range []string{"/api/v1/keys?page=1&page_size=100", "/api/v1/api-keys?page=1&page_size=100"} {
		var res map[string]interface{}
		if err := a.FetchJSON(base+endpoint, "GET", sub2AuthHeaders(accessToken), nil, &res, opt); err != nil {
			continue
		}
		data, err := parseSub2Envelope(res, endpoint)
		if err != nil {
			continue
		}
		tokens := parseSub2TokenItems(data)
		if len(tokens) > 0 {
			return tokens, nil
		}
	}
	return nil, fmt.Errorf("failed to fetch sub2api tokens")
}

func (a *Sub2ApiAdapter) GetApiToken(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (string, error) {
	tokens, err := a.GetApiTokens(baseURL, accessToken, platformUserID, opt)
	if err != nil {
		return "", err
	}
	for _, token := range tokens {
		if token.Enabled && strings.TrimSpace(token.Key) != "" {
			return strings.TrimSpace(token.Key), nil
		}
	}
	if len(tokens) > 0 {
		return strings.TrimSpace(tokens[0].Key), nil
	}
	return "", fmt.Errorf("no valid api token found")
}

func extractSub2ModelIDs(payload interface{}) []string {
	source := payload
	if m, ok := payload.(map[string]interface{}); ok {
		if data, ok := m["data"]; ok {
			source = data
		}
	}
	arr := tokenItemsFromPayload(source)
	if len(arr) == 0 {
		if m, ok := source.(map[string]interface{}); ok {
			if models, ok := m["models"].([]interface{}); ok {
				arr = models
			}
		}
	}
	models := make([]string, 0, len(arr))
	seen := map[string]bool{}
	for _, item := range arr {
		model := ""
		if s, ok := item.(string); ok {
			model = s
		} else if m, ok := item.(map[string]interface{}); ok {
			if id, ok := m["id"].(string); ok {
				model = id
			} else if name, ok := m["name"].(string); ok {
				model = name
			}
		}
		model = strings.TrimSpace(strings.TrimPrefix(model, "models/"))
		if model != "" && !seen[model] {
			seen[model] = true
			models = append(models, model)
		}
	}
	return models
}

func sub2ModelEndpoints(baseURL string) []string {
	base := normalizeSub2BaseURL(baseURL)
	if strings.HasSuffix(strings.ToLower(base), "/models") {
		return []string{base}
	}
	return []string{
		base + "/v1/models",
		base + "/api/v1/models",
		base + "/v1beta/models",
		base + "/antigravity/v1beta/models",
	}
}

func (a *Sub2ApiAdapter) fetchModelsByToken(baseURL, token string, opt *RequestOption) []string {
	authToken := stripBearerPrefix(token)
	if authToken == "" {
		return nil
	}
	for _, endpoint := range sub2ModelEndpoints(baseURL) {
		var res map[string]interface{}
		if err := a.FetchJSON(endpoint, "GET", map[string]string{"Authorization": "Bearer " + authToken}, nil, &res, opt); err != nil {
			continue
		}
		if models := extractSub2ModelIDs(res); len(models) > 0 {
			return models
		}
	}
	return nil
}

func (a *Sub2ApiAdapter) GetModels(baseURL, accessToken string, _ int64, opt *RequestOption) ([]string, error) {
	if models := a.fetchModelsByToken(baseURL, accessToken, opt); len(models) > 0 {
		return models, nil
	}
	apiToken, err := a.GetApiToken(baseURL, accessToken, 0, opt)
	if err == nil && stripBearerPrefix(apiToken) != stripBearerPrefix(accessToken) {
		if models := a.fetchModelsByToken(baseURL, apiToken, opt); len(models) > 0 {
			return models, nil
		}
	}
	return []string{}, nil
}

func (a *Sub2ApiAdapter) VerifyToken(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (*VerifyTokenResult, error) {
	if user, err := fetchSub2AuthMe(a, baseURL, accessToken, opt); err == nil {
		username, _ := user["username"].(string)
		email, _ := user["email"].(string)
		display := strings.TrimSpace(username)
		if display == "" {
			display = strings.TrimSpace(email)
			if at := strings.Index(display, "@"); at > 0 {
				display = display[:at]
			}
		}
		balance, _ := a.GetBalance(baseURL, accessToken, platformUserID, opt)
		apiToken, _ := a.GetApiToken(baseURL, accessToken, platformUserID, opt)
		return &VerifyTokenResult{
			TokenType: "session",
			UserInfo: &UserInfo{
				Username: display,
				Email:    email,
			},
			Balance:  balance,
			ApiToken: apiToken,
		}, nil
	}

	models, err := a.GetModels(baseURL, accessToken, platformUserID, opt)
	if err == nil && len(models) > 0 {
		return &VerifyTokenResult{TokenType: "apikey", Models: models}, nil
	}
	return &VerifyTokenResult{TokenType: "unknown"}, nil
}
