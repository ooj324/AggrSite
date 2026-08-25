package service

import (
	"fmt"
	"log/slog"
	"metapi/aggrsite/db"
	"metapi/aggrsite/platform"
)

type BalanceResult struct {
	Success bool                  `json:"success"`
	Message string                `json:"message,omitempty"`
	Balance *platform.BalanceInfo `json:"balance,omitempty"`
	Skipped bool                  `json:"skipped,omitempty"`
	Reason  string                `json:"reason,omitempty"`
}

func valueOrZero(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func RefreshBalance(accountID int64) (*BalanceResult, error) {
	row, err := db.GetAccountWithSite(accountID)
	if err != nil {
		return nil, fmt.Errorf("account not found: %w", err)
	}

	if row.SiteStatus == "disabled" {
		extraConfig := freshRuntimeHealth(accountID, row.ExtraConfig, "disabled", "站点已禁用", "balance")
		_ = db.UpdateAccount(accountID, map[string]interface{}{"extra_config": extraConfig})
		info := &platform.BalanceInfo{
			Balance: valueOrZero(row.Balance),
			Used:    valueOrZero(row.BalanceUsed),
			Quota:   valueOrZero(row.Quota),
		}
		return &BalanceResult{Success: true, Balance: info, Skipped: true, Reason: "site_disabled"}, nil
	}

	if isApiKeyAccount(row.AccessToken, row.ApiToken, row.ExtraConfig) {
		info := &platform.BalanceInfo{
			Balance: valueOrZero(row.Balance),
			Used:    valueOrZero(row.BalanceUsed),
			Quota:   valueOrZero(row.Quota),
		}
		return &BalanceResult{Success: true, Balance: info, Skipped: true, Reason: "proxy_only"}, nil
	}

	adapter := platform.GetAdapter(row.SitePlatform)
	if adapter == nil {
		return &BalanceResult{Success: false, Message: "unsupported platform: " + row.SitePlatform}, nil
	}

	opt := requestOptionForAccount(*row)

	platformUserID := resolvePlatformUserID(row.ExtraConfig)
	// Managed session pre-refresh if token is nearing expiry (e.g. sub2api, new-api-v1)
	if IsManagedSessionPlatform(row.SitePlatform) {
		if refreshedAccessToken, refreshedExtraConfig, _, err := EnsureManagedSession(*row, opt); err != nil {
			if !isManagedRefreshDeferred(err) {
				slog.Warn("Managed session pre-refresh failed", "account_id", accountID, "platform", row.SitePlatform, "err", err)
			}
		} else {
			row.AccessToken = refreshedAccessToken
			if refreshedExtraConfig != "" {
				row.ExtraConfig = &refreshedExtraConfig
			}
		}
	}

	info, err := adapter.GetBalance(row.SiteURL, row.AccessToken, platformUserID, opt)
	if err != nil {
		slog.Warn("Balance refresh failed, attempting auto-relogin", "account_id", accountID, "err", err)
		if refreshedAccessToken := tryAutoRelogin(*row, adapter, opt); refreshedAccessToken != "" {
			row.AccessToken = refreshedAccessToken
			// Retry balance
			info, err = adapter.GetBalance(row.SiteURL, row.AccessToken, platformUserID, opt)
		}
	}

	if err != nil {
		failure := AnalyzeCheckinFailure(err.Error())
		if isAgentRouterPlatform(row.SitePlatform) && failure.Code == "CLOUDFLARE_CHALLENGE" {
			reason := "AgentRouter balance endpoint is shielded; skipped balance refresh"
			_ = db.UpdateAccount(accountID, map[string]interface{}{
				"extra_config": freshRuntimeHealth(accountID, row.ExtraConfig, "degraded", reason, "balance"),
			})
			info := &platform.BalanceInfo{
				Balance: valueOrZero(row.Balance),
				Used:    valueOrZero(row.BalanceUsed),
				Quota:   valueOrZero(row.Quota),
			}
			return &BalanceResult{Success: true, Balance: info, Skipped: true, Reason: "agentrouter_balance_shielded", Message: reason}, nil
		}

		slog.Warn("Balance refresh failed completely", "account_id", accountID, "err", err)
		statusUpdate := map[string]interface{}{
			"extra_config": freshRuntimeHealth(accountID, row.ExtraConfig, "unhealthy", err.Error(), "balance"),
		}
		if failure.Code == "TOKEN_EXPIRED" {
			statusUpdate["status"] = "expired"
		}
		_ = db.UpdateAccount(accountID, statusUpdate)
		return &BalanceResult{Success: false, Message: err.Error()}, nil
	}

	// Persist to DB
	updates := map[string]interface{}{
		"balance":              info.Balance,
		"balance_used":         info.Used,
		"quota":                info.Quota,
		"last_balance_refresh": db.TimeNow(),
		"extra_config":         freshRuntimeHealth(accountID, row.ExtraConfig, "healthy", "余额刷新成功", "balance"),
	}
	if row.Status != nil && *row.Status == "expired" {
		updates["status"] = "active"
	}
	_ = db.UpdateAccount(accountID, updates)

	slog.Info("Balance refreshed", "account_id", accountID,
		"balance", info.Balance, "used", info.Used, "quota", info.Quota)

	return &BalanceResult{Success: true, Balance: info}, nil
}

type RefreshAllResult struct {
	AccountID int64          `json:"account_id"`
	Username  string         `json:"username"`
	Site      string         `json:"site"`
	Result    *BalanceResult `json:"result"`
}

func RefreshAllBalances() ([]RefreshAllResult, error) {
	accounts, err := db.ListBalanceRefreshableAccounts()
	if err != nil {
		return nil, err
	}

	var results []RefreshAllResult
	for _, row := range accounts {
		r, _ := RefreshBalance(row.ID)
		if r == nil {
			r = &BalanceResult{Success: false, Message: "internal error"}
		}
		results = append(results, RefreshAllResult{
			AccountID: row.ID,
			Username:  nullStr(row.Username),
			Site:      row.SiteName,
			Result:    r,
		})
	}

	return results, nil
}
