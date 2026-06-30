package platform

import "fmt"

// OneApiAdapter handles one-api compatible platforms.
// Balance calculation: balance = quota - used (different from new-api).
type OneApiAdapter struct {
	BaseAdapter
}

func init() {
	Register(&OneApiAdapter{BaseAdapter: BaseAdapter{Name: "one-api"}})
}

func (a *OneApiAdapter) GetApiToken(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (string, error) {
	return getApiTokenWithSessionCookie(&a.BaseAdapter, baseURL, accessToken, platformUserID, opt)
}

func (a *OneApiAdapter) GetApiTokens(baseURL, accessToken string, platformUserID int64, opt *RequestOption) ([]ApiTokenInfo, error) {
	return getApiTokensWithSessionCookie(&a.BaseAdapter, baseURL, accessToken, platformUserID, opt)
}

func (a *OneApiAdapter) Checkin(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (*CheckinResult, error) {
	url := fmt.Sprintf("%s/api/user/checkin", baseURL)

	var res map[string]interface{}
	usedCookie, err := fetchJSONWithSessionCookie(
		url,
		"POST",
		accessToken,
		platformUserID,
		map[string]string{"X-Requested-With": "XMLHttpRequest"},
		map[string]interface{}{},
		&res,
		opt,
	)
	if !usedCookie {
		err = a.FetchJSON(url, "POST", AuthHeaders(accessToken, platformUserID), nil, &res, opt)
	}
	if err != nil {
		return &CheckinResult{Success: false, Message: err.Error()}, nil
	}

	success, _ := res["success"].(bool)
	message := ExtractMessage(res)
	if message == "" {
		if success {
			message = "Check-in successful"
		} else {
			message = "Check-in failed"
		}
	}

	reward := ""
	if data, ok := res["data"].(map[string]interface{}); ok {
		if r, ok := data["reward"]; ok {
			reward = fmt.Sprintf("%v", r)
		}
	}

	return &CheckinResult{Success: success, Message: message, Reward: reward}, nil
}

func (a *OneApiAdapter) GetBalance(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (*BalanceInfo, error) {
	url := fmt.Sprintf("%s/api/user/self", baseURL)

	var res map[string]interface{}
	usedCookie, err := fetchJSONWithSessionCookie(url, "GET", accessToken, platformUserID, nil, nil, &res, opt)
	if !usedCookie {
		err = a.FetchJSON(url, "GET", AuthHeaders(accessToken, platformUserID), nil, &res, opt)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch balance: %w", err)
	}

	data, _ := res["data"].(map[string]interface{})
	if data == nil {
		return nil, fmt.Errorf("no data in balance response")
	}

	quota := toFloat(data["quota"]) / 500000
	used := toFloat(data["used_quota"]) / 500000
	return &BalanceInfo{
		Balance: quota - used,
		Used:    used,
		Quota:   quota,
	}, nil
}

func (a *OneApiAdapter) VerifyToken(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (*VerifyTokenResult, error) {
	userURL := fmt.Sprintf("%s/api/user/self", baseURL)
	var userRes map[string]interface{}

	usedCookie, err := fetchJSONWithSessionCookie(userURL, "GET", accessToken, platformUserID, nil, nil, &userRes, opt)
	if !usedCookie {
		err = a.FetchJSON(userURL, "GET", AuthHeaders(accessToken, platformUserID), nil, &userRes, opt)
	}
	if err == nil {
		success, _ := userRes["success"].(bool)
		if success {
			ui, _ := parseUserInfoAndBalance(userRes["data"])
			balance, _ := a.GetBalance(baseURL, accessToken, platformUserID, opt)
			apiToken, _ := a.GetApiToken(baseURL, accessToken, platformUserID, opt)
			return &VerifyTokenResult{
				TokenType: "session",
				UserInfo:  ui,
				Balance:   balance,
				ApiToken:  apiToken,
			}, nil
		}
	}

	if usedCookie {
		return &VerifyTokenResult{TokenType: "unknown"}, nil
	}

	models, err := a.GetModels(baseURL, accessToken, platformUserID, opt)
	if err == nil && len(models) > 0 {
		return &VerifyTokenResult{TokenType: "apikey", Models: models}, nil
	}

	return &VerifyTokenResult{TokenType: "unknown"}, nil
}
