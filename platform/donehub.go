package platform

import "fmt"

// DoneHubAdapter: checkin not supported; balance = quotaRemaining + used.
type DoneHubAdapter struct {
	OneHubAdapter
}

func init() {
	Register(&DoneHubAdapter{OneHubAdapter: OneHubAdapter{OneApiAdapter: OneApiAdapter{BaseAdapter: BaseAdapter{Name: "done-hub"}}}})
}

func (a *DoneHubAdapter) Checkin(_ string, _ string, _ int64, _ *RequestOption) (*CheckinResult, error) {
	return &CheckinResult{Success: false, Message: "checkin endpoint not found"}, nil
}

func (a *DoneHubAdapter) GetBalance(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (*BalanceInfo, error) {
	url := fmt.Sprintf("%s/api/user/self", baseURL)
	headers := AuthHeaders(accessToken, platformUserID)

	var res map[string]interface{}
	err := a.FetchJSON(url, "GET", headers, nil, &res, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch balance: %w", err)
	}

	data, _ := res["data"].(map[string]interface{})
	if data == nil {
		// DoneHub sometimes returns data at root
		data = res
	}

	quotaRemaining := toFloat(data["quota"]) / 500000
	used := toFloat(data["used_quota"]) / 500000
	total := quotaRemaining + used

	return &BalanceInfo{
		Balance: quotaRemaining,
		Used:    used,
		Quota:   total,
	}, nil
}

func (a *DoneHubAdapter) PlatformName() string { return "done-hub" }

func (a *DoneHubAdapter) VerifyToken(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (*VerifyTokenResult, error) {
	userURL := fmt.Sprintf("%s/api/user/self", baseURL)
	var userRes map[string]interface{}
	if err := a.FetchJSON(userURL, "GET", AuthHeaders(accessToken, platformUserID), nil, &userRes, opt); err == nil {
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

	models, err := a.GetModels(baseURL, accessToken, platformUserID, opt)
	if err == nil && len(models) > 0 {
		return &VerifyTokenResult{TokenType: "apikey", Models: models}, nil
	}

	return &VerifyTokenResult{TokenType: "unknown"}, nil
}
