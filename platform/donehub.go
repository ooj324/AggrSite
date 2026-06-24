package platform

import "fmt"

// DoneHubAdapter: checkin not supported; balance = quotaRemaining + used.
type DoneHubAdapter struct {
	BaseAdapter
}

func init() {
	Register(&DoneHubAdapter{BaseAdapter: BaseAdapter{Name: "done-hub"}})
}

func (a *DoneHubAdapter) Checkin(_ string, _ string, _ int64) (*CheckinResult, error) {
	return &CheckinResult{Success: false, Message: "checkin endpoint not found"}, nil
}

func (a *DoneHubAdapter) GetBalance(baseURL, accessToken string, _ int64) (*BalanceInfo, error) {
	url := fmt.Sprintf("%s/api/user/self", baseURL)
	headers := map[string]string{"Authorization": "Bearer " + accessToken}

	var res map[string]interface{}
	err := a.FetchJSON(url, "GET", headers, nil, &res)
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
