package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"metapi/aggrsite/db"
	"metapi/aggrsite/platform"
	"strings"
)

// ---- Helpers ----

func nullStr(ns *string) string {
	if ns != nil {
		return *ns
	}
	return ""
}

func resolvePlatformUserID(extraConfig *string) int64 {
	if extraConfig == nil || *extraConfig == "" {
		return 0
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(*extraConfig), &cfg); err != nil {
		return 0
	}
	if id, ok := cfg["platformUserId"].(float64); ok && id > 0 {
		return int64(id)
	}
	return 0
}

// ---- Checkin ----

type CheckinAccountResult struct {
	Success bool   `json:"success"`
	Status  string `json:"status"` // success | failed | skipped
	Message string `json:"message"`
	Reward  string `json:"reward,omitempty"`
}

func isAlreadyCheckedIn(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	patterns := []string{
		"already checked in", "already signed", "already sign in",
		"今日已签到", "今天已签到", "今天已经签到", "今日已经签到",
		"已经签到", "已签到", "重复签到", "签到过",
	}
	for _, p := range patterns {
		if strings.Contains(text, p) {
			return true
		}
	}
	return false
}

func isUnsupportedCheckin(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	patterns := []string{
		"invalid url (post /api/user/checkin)",
		"checkin endpoint not found",
		"check-in is not supported",
		"checkin is not supported",
		"does not support checkin",
		"not support checkin",
	}
	for _, p := range patterns {
		if strings.Contains(text, p) {
			return true
		}
	}
	if strings.Contains(text, "http 404") && strings.Contains(text, "/api/user/checkin") {
		return true
	}
	return false
}

func CheckinAccount(accountID int64) (*CheckinAccountResult, error) {
	row, err := db.GetAccountWithSite(accountID)
	if err != nil {
		return nil, fmt.Errorf("account not found: %w", err)
	}

	// Skip disabled sites
	if row.SiteStatus == "disabled" {
		_ = db.InsertCheckinLog(accountID, "skipped", "site disabled", "")
		slog.Info("Checkin skipped: site disabled", "account_id", accountID)
		return &CheckinAccountResult{Success: true, Status: "skipped", Message: "site disabled"}, nil
	}

	adapter := platform.GetAdapter(row.SitePlatform)
	if adapter == nil {
		msg := "unsupported platform: " + row.SitePlatform
		_ = db.InsertCheckinLog(accountID, "skipped", msg, "")
		return &CheckinAccountResult{Success: false, Status: "skipped", Message: msg}, nil
	}

	// Check if external checkin URL is provided
	checkinURL := row.SiteURL
	if row.SiteExternalCheckinURL != nil && *row.SiteExternalCheckinURL != "" {
		checkinURL = *row.SiteExternalCheckinURL
		slog.Info("Using external checkin URL", "account_id", accountID, "url", checkinURL)
	}

	opt := &platform.RequestOption{
		ProxyURL:       row.SiteProxyURL,
		UseSystemProxy: row.SiteUseSystemProxy,
		CustomHeaders:  row.SiteCustomHeaders,
	}
	if row.ExtraConfig != nil && *row.ExtraConfig != "" {
		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(*row.ExtraConfig), &cfg); err == nil {
			if proxyUrl, ok := cfg["proxyUrl"].(string); ok && proxyUrl != "" {
				opt.ProxyURL = &proxyUrl
			}
			if useSystemProxy, ok := cfg["useSystemProxy"].(bool); ok {
				opt.UseSystemProxy = &useSystemProxy
			}
		}
	}

	platformUserID := resolvePlatformUserID(row.ExtraConfig)
	result, err := adapter.Checkin(checkinURL, row.AccessToken, platformUserID, opt)
	if err != nil {
		result = &platform.CheckinResult{Success: false, Message: err.Error()}
	}

	alreadyCheckedIn := isAlreadyCheckedIn(result.Message)
	unsupported := isUnsupportedCheckin(result.Message)
	
	failureReason := AnalyzeCheckinFailure(result.Message)
	turnstileRequired := failureReason.Code == "TURNSTILE_REQUIRED"

	effectiveSuccess := result.Success || alreadyCheckedIn || unsupported || turnstileRequired

	var status string
	switch {
	case unsupported || turnstileRequired:
		status = "skipped"
	case effectiveSuccess:
		status = "success"
	default:
		// Attempt auto-relogin ONLY if token is expired
		if failureReason.Code == "TOKEN_EXPIRED" {
			if refreshedAccessToken := tryAutoRelogin(*row, adapter, opt); refreshedAccessToken != "" {
				row.AccessToken = refreshedAccessToken
				// Retry checkin
				result, err = adapter.Checkin(checkinURL, row.AccessToken, platformUserID, opt)
				if err != nil {
					result = &platform.CheckinResult{Success: false, Message: err.Error()}
				}
				
				alreadyCheckedIn = isAlreadyCheckedIn(result.Message)
				unsupported = isUnsupportedCheckin(result.Message)
				newFailure := AnalyzeCheckinFailure(result.Message)
				turnstileRequired = newFailure.Code == "TURNSTILE_REQUIRED"
				
				effectiveSuccess = result.Success || alreadyCheckedIn || unsupported || turnstileRequired
				if unsupported || turnstileRequired {
					status = "skipped"
				} else if effectiveSuccess {
					status = "success"
				} else {
					status = "failed"
				}
			} else {
				status = "failed"
			}
		} else {
			status = "failed"
		}
	}

	// Reward inference if success but no explicit reward
	if result.Success && !alreadyCheckedIn && !unsupported && !turnstileRequired && result.Reward == "" {
		if preBalance, err := adapter.GetBalance(checkinURL, row.AccessToken, platformUserID, opt); err == nil {
			postBalance, err2 := RefreshBalance(accountID)
			if err2 == nil && postBalance != nil && postBalance.Balance != nil {
				delta := postBalance.Balance.Quota - preBalance.Quota
				if delta > 0 {
					result.Reward = fmt.Sprintf("推断奖励: %.2f", delta)
				}
			}
		}
	}

	// Write checkin log
	_ = db.InsertCheckinLog(accountID, status, result.Message, result.Reward)

	// Write event
	eventLevel := "info"
	eventTitle := "checkin " + status
	if status == "failed" {
		eventLevel = "error"
	}
	_ = db.InsertEvent("checkin", eventTitle,
		fmt.Sprintf("%s @ %s: %s", nullStr(row.Username), row.SiteName, result.Message),
		eventLevel, &accountID, "account")

	// Update last_checkin_at on success
	if result.Success && !alreadyCheckedIn && !unsupported && !turnstileRequired {
		_ = db.UpdateAccount(accountID, map[string]interface{}{
			"last_checkin_at": db.TimeNow(),
		})
	}

	// Try refreshing balance on success (if not already done by inference)
	if effectiveSuccess && !unsupported && !turnstileRequired && result.Reward == "" {
		go func() {
			_, _ = RefreshBalance(accountID)
		}()
	}

	slog.Info("Checkin completed", "account_id", accountID, "status", status, "message", result.Message)
	return &CheckinAccountResult{
		Success: effectiveSuccess,
		Status:  status,
		Message: result.Message,
		Reward:  result.Reward,
	}, nil
}

func tryAutoRelogin(row db.AccountWithSite, adapter platform.Adapter, opt *platform.RequestOption) string {
	if row.ExtraConfig == nil || *row.ExtraConfig == "" {
		return ""
	}
	
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(*row.ExtraConfig), &cfg); err != nil {
		return ""
	}
	
	autoRelogin, ok := cfg["autoRelogin"].(map[string]interface{})
	if !ok {
		return ""
	}
	
	username, _ := autoRelogin["username"].(string)
	passwordCipher, _ := autoRelogin["passwordCipher"].(string)
	
	if username == "" || passwordCipher == "" {
		return ""
	}
	
	password := DecryptPassword(passwordCipher)
	if password == "" {
		return ""
	}
	
	slog.Info("Attempting auto-relogin", "account_id", row.ID)
	loginResult, err := adapter.Login(row.SiteURL, username, password, opt)
	if err != nil || loginResult == nil || !loginResult.Success || loginResult.AccessToken == "" {
		slog.Warn("Auto-relogin failed", "account_id", row.ID, "err", err, "message", loginResult.Message)
		return ""
	}
	
	slog.Info("Auto-relogin successful, updating access token", "account_id", row.ID)
	_ = db.UpdateAccount(row.ID, map[string]interface{}{
		"access_token": loginResult.AccessToken,
	})
	
	return loginResult.AccessToken
}

type CheckinAllResult struct {
	AccountID int64                `json:"account_id"`
	Username  string               `json:"username"`
	Site      string               `json:"site"`
	Result    *CheckinAccountResult `json:"result"`
}

func CheckinAll() ([]CheckinAllResult, error) {
	rows, err := db.ListCheckinableAccounts()
	if err != nil {
		return nil, err
	}

	var results []CheckinAllResult
	for _, row := range rows {
		r, _ := CheckinAccount(row.ID)
		if r == nil {
			r = &CheckinAccountResult{Success: false, Status: "failed", Message: "internal error"}
		}
		results = append(results, CheckinAllResult{
			AccountID: row.ID,
			Username:  nullStr(row.Username),
			Site:      row.SiteName,
			Result:    r,
		})
	}

	return results, nil
}
