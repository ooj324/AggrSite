package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"metapi/aggrsite/db"
	"metapi/aggrsite/platform"
)

type BalanceResult struct {
	Success bool               `json:"success"`
	Message string             `json:"message,omitempty"`
	Balance *platform.BalanceInfo `json:"balance,omitempty"`
}

func RefreshBalance(accountID int64) (*BalanceResult, error) {
	row, err := db.GetAccountWithSite(accountID)
	if err != nil {
		return nil, fmt.Errorf("account not found: %w", err)
	}

	adapter := platform.GetAdapter(row.SitePlatform)
	if adapter == nil {
		return &BalanceResult{Success: false, Message: "unsupported platform: " + row.SitePlatform}, nil
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
		return &BalanceResult{Success: false, Message: err.Error()}, nil
	}

	// Persist to DB
	_ = db.UpdateAccount(accountID, map[string]interface{}{
		"balance":              info.Balance,
		"balance_used":         info.Used,
		"quota":                info.Quota,
		"last_balance_refresh": db.TimeNow(),
	})

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
		SELECT a.*, s.name AS site_name, s.url AS site_url, s.platform AS site_platform, s.status AS site_status, s.proxy_url AS site_proxy_url, s.use_system_proxy AS site_use_system_proxy, s.external_checkin_url AS site_external_checkin_url, s.custom_headers AS site_custom_headers
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
