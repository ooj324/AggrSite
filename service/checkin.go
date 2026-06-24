package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"metapi/aggrsite/db"
	"metapi/aggrsite/platform"
	"strings"
)

// ---- Helpers ----

func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func resolvePlatformUserID(extraConfig sql.NullString) int64 {
	if !extraConfig.Valid || extraConfig.String == "" {
		return 0
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(extraConfig.String), &cfg); err != nil {
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

	platformUserID := resolvePlatformUserID(row.ExtraConfig)
	result, err := adapter.Checkin(row.SiteURL, row.AccessToken, platformUserID)
	if err != nil {
		result = &platform.CheckinResult{Success: false, Message: err.Error()}
	}

	alreadyCheckedIn := isAlreadyCheckedIn(result.Message)
	unsupported := isUnsupportedCheckin(result.Message)
	effectiveSuccess := result.Success || alreadyCheckedIn || unsupported

	var status string
	switch {
	case unsupported:
		status = "skipped"
	case effectiveSuccess:
		status = "success"
	default:
		status = "failed"
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
	if result.Success && !alreadyCheckedIn && !unsupported {
		_ = db.UpdateAccount(accountID, map[string]interface{}{
			"last_checkin_at": db.TimeNow(),
		})
	}

	// Also try refreshing balance on success
	if effectiveSuccess && !unsupported {
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
