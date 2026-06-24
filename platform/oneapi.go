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

func (a *OneApiAdapter) Checkin(baseURL, accessToken string, _ int64, opt *RequestOption) (*CheckinResult, error) {
	url := fmt.Sprintf("%s/api/user/checkin", baseURL)
	headers := map[string]string{"Authorization": "Bearer " + accessToken}

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

func (a *OneApiAdapter) GetBalance(baseURL, accessToken string, _ int64, opt *RequestOption) (*BalanceInfo, error) {
	url := fmt.Sprintf("%s/api/user/self", baseURL)
	headers := map[string]string{"Authorization": "Bearer " + accessToken}

	var res map[string]interface{}
	err := a.FetchJSON(url, "GET", headers, nil, &res, opt)
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

