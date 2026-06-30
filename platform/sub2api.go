package platform

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
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

func (a *Sub2ApiAdapter) RefreshAuth(baseURL, accessToken, extraConfig string, opt *RequestOption) (*RefreshResult, error) {
	if extraConfig == "" {
		return &RefreshResult{Success: false, Message: "No extraConfig provided"}, nil
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(extraConfig), &cfg); err != nil {
		return &RefreshResult{Success: false, Message: "Invalid extraConfig format"}, nil
	}

	sub2apiAuth, ok := cfg["sub2apiAuth"].(map[string]interface{})
	if !ok || sub2apiAuth == nil {
		return &RefreshResult{Success: false, Message: "No sub2apiAuth found in extraConfig"}, nil
	}

	refreshToken, _ := sub2apiAuth["refreshToken"].(string)
	if refreshToken == "" {
		return &RefreshResult{Success: false, Message: "No refreshToken found in sub2apiAuth"}, nil
	}

	base := normalizeSub2BaseURL(baseURL)
	url := fmt.Sprintf("%s/api/v1/auth/refresh", base)

	headers := map[string]string{}
	token := stripBearerPrefix(accessToken)
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}

	body := map[string]string{
		"refresh_token": refreshToken,
	}

	fetchRefresh := func(headers map[string]string) (map[string]interface{}, error) {
		var res map[string]interface{}
		if err := a.FetchJSON(url, "POST", headers, body, &res, opt); err != nil {
			return nil, err
		}
		return res, nil
	}

	res, err := fetchRefresh(headers)
	if err != nil && token != "" {
		res, err = fetchRefresh(nil)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch refresh: %w", err)
	}

	dataRaw, err := parseSub2Envelope(res, "/api/v1/auth/refresh")
	if err != nil {
		return nil, err
	}
	data, _ := dataRaw.(map[string]interface{})
	if data == nil {
		return nil, fmt.Errorf("no data in refresh response")
	}

	newAccessToken, _ := data["access_token"].(string)
	newRefreshToken, _ := data["refresh_token"].(string)
	expiresIn, _ := data["expires_in"].(float64)

	if newAccessToken == "" || newRefreshToken == "" || expiresIn <= 0 {
		return nil, fmt.Errorf("invalid token data in refresh response")
	}

	tokenExpiresAt := time.Now().UnixMilli() + int64(expiresIn*1000)

	sub2apiAuth["refreshToken"] = newRefreshToken
	sub2apiAuth["tokenExpiresAt"] = tokenExpiresAt
	cfg["sub2apiAuth"] = sub2apiAuth

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
