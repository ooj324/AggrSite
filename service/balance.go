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
		extraConfig := mergeRuntimeHealth(row.ExtraConfig, "disabled", "站点已禁用", "balance")
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
	if isSub2APIPlatform(row.SitePlatform) {
		if refreshedAccessToken, refreshedExtraConfig, _, err := ensureSub2APIManagedSession(*row, opt); err != nil {
			slog.Warn("Sub2API managed session pre-refresh failed", "account_id", accountID, "err", err)
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
		slog.Warn("Balance refresh failed completely", "account_id", accountID, "err", err)
		statusUpdate := map[string]interface{}{
			"extra_config": mergeRuntimeHealth(row.ExtraConfig, "unhealthy", err.Error(), "balance"),
		}
		if AnalyzeCheckinFailure(err.Error()).Code == "TOKEN_EXPIRED" {
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
		"extra_config":         mergeRuntimeHealth(row.ExtraConfig, "healthy", "余额刷新成功", "balance"),
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
	var accounts []db.AccountWithSite
	err := db.DB.Select(&accounts, `
		SELECT a.id, a.site_id, a.username, a.access_token, a.api_token, a.balance, a.balance_used, a.quota, a.unit_cost, a.value_score, a.status, a.checkin_enabled, a.last_checkin_at, a.last_balance_refresh, a.extra_config, a.created_at, a.updated_at, a.is_pinned, a.sort_order, s.name AS site_name, s.url AS site_url, s.platform AS site_platform, s.status AS site_status, s.proxy_url AS site_proxy_url, s.use_system_proxy AS site_use_system_proxy, s.external_checkin_url AS site_external_checkin_url, s.custom_headers AS site_custom_headers
		FROM accounts a
		INNER JOIN sites s ON a.site_id = s.id
		WHERE a.status = 'active'
		ORDER BY a.id ASC
	`)
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
