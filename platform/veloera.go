package platform

import "fmt"

// VeloeraAdapter: same as new-api but divisor is 1_000_000 instead of 500_000.
type VeloeraAdapter struct {
	BaseAdapter
}

func init() {
	Register(&VeloeraAdapter{BaseAdapter: BaseAdapter{Name: "veloera"}})
}

func (a *VeloeraAdapter) Checkin(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (*CheckinResult, error) {
	url := fmt.Sprintf("%s/api/user/checkin", baseURL)
	headers := AuthHeaders(accessToken, platformUserID)

	var res map[string]interface{}
	err := a.FetchJSON(url, "POST", headers, nil, &res, opt)
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

func (a *VeloeraAdapter) GetBalance(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (*BalanceInfo, error) {
	url := fmt.Sprintf("%s/api/user/self", baseURL)
	headers := AuthHeaders(accessToken, platformUserID)

	var res map[string]interface{}
	err := a.FetchJSON(url, "GET", headers, nil, &res, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch balance: %w", err)
	}

	data, _ := res["data"].(map[string]interface{})
	if data == nil {
		return nil, fmt.Errorf("no data in balance response")
	}

	// Veloera uses 1_000_000 as divisor
	quota := toFloat(data["quota"]) / 1000000
	used := toFloat(data["used_quota"]) / 1000000
	return &BalanceInfo{
		Balance: quota - used,
		Used:    used,
		Quota:   quota,
	}, nil
}

func (a *VeloeraAdapter) VerifyToken(baseURL, accessToken string, platformUserID int64, opt *RequestOption) (*VerifyTokenResult, error) {
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
