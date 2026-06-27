package platform

import (
	"encoding/json"
	"fmt"
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

	base := strings.TrimRight(baseURL, "/")
	url := fmt.Sprintf("%s/api/v1/auth/refresh", base)
	
	headers := map[string]string{}
	token := strings.TrimPrefix(strings.TrimSpace(accessToken), "Bearer ")
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}

	body := map[string]string{
		"refresh_token": refreshToken,
	}

	var res map[string]interface{}
	err := a.FetchJSON(url, "POST", headers, body, &res, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch refresh: %w", err)
	}

	code, _ := res["code"].(float64)
	if code != 0 {
		msg := ExtractMessage(res)
		if msg == "" {
			msg = fmt.Sprintf("Refresh Error code %v", code)
		}
		return nil, fmt.Errorf(msg)
	}

	data, _ := res["data"].(map[string]interface{})
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

func (a *Sub2ApiAdapter) GetApiToken(_ string, _ string, _ int64, _ *RequestOption) (string, error) {
	return "", fmt.Errorf("API Tokens are not supported by Sub2API")
}
