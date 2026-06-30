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
	return verifyTokenWithCookieFallback(a, &a.OneHubAdapter.OneApiAdapter.BaseAdapter, baseURL, accessToken, platformUserID, opt)
}
