package platform

import (
	"fmt"
	"strings"
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

func (a *Sub2ApiAdapter) GetBalance(baseURL, accessToken string, _ int64, opt *RequestOption) (*BalanceInfo, error) {
	base := strings.TrimRight(baseURL, "/")
	token := strings.TrimPrefix(strings.TrimSpace(accessToken), "Bearer ")

	url := fmt.Sprintf("%s/api/v1/auth/me", base)
	headers := map[string]string{"Authorization": "Bearer " + token}

	var res map[string]interface{}
	err := a.FetchJSON(url, "GET", headers, nil, &res, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch balance: %w", err)
	}

	// Sub2API envelope: { code: 0, data: { balance: ... } }
	code, _ := res["code"].(float64)
	if code != 0 {
		msg := ExtractMessage(res)
		if msg == "" {
			msg = fmt.Sprintf("Error code %v", code)
		}
		return nil, fmt.Errorf(msg)
	}

	data, _ := res["data"].(map[string]interface{})
	if data == nil {
		return nil, fmt.Errorf("no data in auth/me response")
	}

	balance := toFloat(data["balance"])
	// Sub2API balance is already in USD; convert to internal quota unit and back
	quotaValue := balance * 500000
	return &BalanceInfo{
		Balance: quotaValue / 500000,
		Used:    0,
		Quota:   quotaValue / 500000,
	}, nil
}

func (a *Sub2ApiAdapter) Login(_ string, _ string, _ string, _ *RequestOption) (*LoginResult, error) {
	return &LoginResult{Success: false, Message: "Login is not supported by Sub2API"}, nil
}

func (a *Sub2ApiAdapter) GetApiToken(_ string, _ string, _ int64, _ *RequestOption) (string, error) {
	return "", fmt.Errorf("API Tokens are not supported by Sub2API")
}
