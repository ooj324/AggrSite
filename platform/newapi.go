package platform

import (
	"fmt"
	"math"
)

// NewApiAdapter handles new-api compatible platforms.
type NewApiAdapter struct {
	BaseAdapter
}

func init() {
	Register(&NewApiAdapter{BaseAdapter: BaseAdapter{Name: "new-api"}})
}

func (a *NewApiAdapter) Checkin(baseURL, accessToken string, platformUserID int64) (*CheckinResult, error) {
	url := fmt.Sprintf("%s/api/user/checkin", baseURL)
	headers := AuthHeaders(accessToken, platformUserID)

	var res map[string]interface{}
	err := a.FetchJSON(url, "POST", headers, nil, &res)
	if err != nil {
		return &CheckinResult{Success: false, Message: err.Error()}, nil
	}

	success, _ := res["success"].(bool)
	message := ExtractMessage(res)
	if message == "" {
		if success {
			message = "checkin success"
		} else {
			message = "checkin failed"
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

func (a *NewApiAdapter) GetBalance(baseURL, accessToken string, platformUserID int64) (*BalanceInfo, error) {
	url := fmt.Sprintf("%s/api/user/self", baseURL)
	headers := AuthHeaders(accessToken, platformUserID)

	var res map[string]interface{}
	err := a.FetchJSON(url, "GET", headers, nil, &res)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch balance: %w", err)
	}

	success, _ := res["success"].(bool)
	if !success {
		return nil, fmt.Errorf("failed to fetch balance: %s", ExtractMessage(res))
	}

	data, _ := res["data"].(map[string]interface{})
	if data == nil {
		return nil, fmt.Errorf("no data in balance response")
	}

	return parseNewApiBalance(data, 500000), nil
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
