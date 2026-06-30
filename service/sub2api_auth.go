package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"metapi/aggrsite/db"
	"metapi/aggrsite/platform"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	sub2APIPlatform               = "sub2api"
	sub2APIManagedRefreshLead     = 10 * time.Minute
	sub2APIManagedRefreshInterval = 5 * time.Minute
)

var (
	sub2APIRefreshMu     sync.Map
	sub2APIRefreshLoopMu sync.Mutex
	sub2APIRefreshStopCh chan struct{}
	sub2APIRefreshPassMu sync.Mutex
)

type sub2APIAuthState struct {
	RefreshToken   string
	TokenExpiresAt int64
}

func isSub2APIPlatform(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), sub2APIPlatform)
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func requestOptionForAccount(row db.AccountWithSite) *platform.RequestOption {
	opt := &platform.RequestOption{
		ProxyURL:       row.SiteProxyURL,
		UseSystemProxy: row.SiteUseSystemProxy,
		CustomHeaders:  row.SiteCustomHeaders,
	}
	if row.ExtraConfig == nil || *row.ExtraConfig == "" {
		return opt
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(*row.ExtraConfig), &cfg); err != nil {
		return opt
	}
	if proxyURL, ok := cfg["proxyUrl"].(string); ok && proxyURL != "" {
		opt.ProxyURL = &proxyURL
	}
	if useSystemProxy, ok := cfg["useSystemProxy"].(bool); ok {
		opt.UseSystemProxy = &useSystemProxy
	}
	return opt
}

func parseSub2APITokenExpiresAt(raw interface{}) int64 {
	var value int64
	switch v := raw.(type) {
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
			return 0
		}
		value = int64(v)
	case int64:
		value = v
	case int:
		value = int64(v)
	case json.Number:
		parsed, err := v.Int64()
		if err != nil {
			return 0
		}
		value = parsed
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return 0
		}
		value = parsed
	default:
		return 0
	}
	if value > 0 && value < 1_000_000_000_000 {
		return value * 1000
	}
	return value
}

func getSub2APIAuthState(extraConfig *string) sub2APIAuthState {
	if extraConfig == nil || *extraConfig == "" {
		return sub2APIAuthState{}
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(*extraConfig), &cfg); err != nil {
		return sub2APIAuthState{}
	}
	auth, _ := cfg["sub2apiAuth"].(map[string]interface{})
	if auth == nil {
		return sub2APIAuthState{}
	}
	refreshToken, _ := auth["refreshToken"].(string)
	return sub2APIAuthState{
		RefreshToken:   strings.TrimSpace(refreshToken),
		TokenExpiresAt: parseSub2APITokenExpiresAt(auth["tokenExpiresAt"]),
	}
}

func isSub2APIManagedTokenDue(expiresAt int64, now time.Time) bool {
	if expiresAt <= 0 {
		return false
	}
	return expiresAt-now.UnixMilli() <= int64(sub2APIManagedRefreshLead/time.Millisecond)
}

func getSub2APIRefreshMutex(accountID int64) *sync.Mutex {
	mu, _ := sub2APIRefreshMu.LoadOrStore(accountID, &sync.Mutex{})
	return mu.(*sync.Mutex)
}

func refreshSub2APIManagedSession(row db.AccountWithSite, opt *platform.RequestOption, force bool) (string, string, bool, error) {
	if !isSub2APIPlatform(row.SitePlatform) {
		return row.AccessToken, stringValue(row.ExtraConfig), false, nil
	}

	state := getSub2APIAuthState(row.ExtraConfig)
	if state.RefreshToken == "" {
		return row.AccessToken, stringValue(row.ExtraConfig), false, nil
	}
	if !force && !isSub2APIManagedTokenDue(state.TokenExpiresAt, time.Now()) {
		return row.AccessToken, stringValue(row.ExtraConfig), false, nil
	}

	mu := getSub2APIRefreshMutex(row.ID)
	mu.Lock()
	defer mu.Unlock()

	latest, err := db.GetAccountWithSite(row.ID)
	if err != nil {
		return row.AccessToken, stringValue(row.ExtraConfig), false, err
	}
	latestState := getSub2APIAuthState(latest.ExtraConfig)
	if latestState.RefreshToken == "" {
		return latest.AccessToken, stringValue(latest.ExtraConfig), false, nil
	}

	latestExtraConfig := stringValue(latest.ExtraConfig)
	if force && latest.AccessToken != row.AccessToken {
		return latest.AccessToken, latestExtraConfig, false, nil
	}
	if !force && !isSub2APIManagedTokenDue(latestState.TokenExpiresAt, time.Now()) {
		return latest.AccessToken, latestExtraConfig, false, nil
	}

	adapter := platform.GetAdapter(latest.SitePlatform)
	if adapter == nil {
		return latest.AccessToken, latestExtraConfig, false, fmt.Errorf("unsupported platform: %s", latest.SitePlatform)
	}
	if opt == nil {
		opt = requestOptionForAccount(*latest)
	}

	res, err := adapter.RefreshAuth(latest.SiteURL, latest.AccessToken, latestExtraConfig, opt)
	if err != nil {
		return latest.AccessToken, latestExtraConfig, false, err
	}
	if res == nil || !res.Success || strings.TrimSpace(res.AccessToken) == "" {
		message := "sub2api refresh failed"
		if res != nil && strings.TrimSpace(res.Message) != "" {
			message = res.Message
		}
		return latest.AccessToken, latestExtraConfig, false, fmt.Errorf("%s", message)
	}

	updates := map[string]interface{}{
		"access_token": res.AccessToken,
		"extra_config": res.ExtraConfig,
	}
	if latest.Status != nil && *latest.Status == "expired" {
		updates["status"] = "active"
	}
	if err := db.UpdateAccount(latest.ID, updates); err != nil {
		return latest.AccessToken, latestExtraConfig, false, err
	}

	slog.Info("Sub2API managed session refreshed", "account_id", latest.ID)
	return res.AccessToken, res.ExtraConfig, true, nil
}

func ensureSub2APIManagedSession(row db.AccountWithSite, opt *platform.RequestOption) (string, string, bool, error) {
	return refreshSub2APIManagedSession(row, opt, false)
}

func forceRefreshSub2APIManagedSession(row db.AccountWithSite, opt *platform.RequestOption) (string, string, bool, error) {
	return refreshSub2APIManagedSession(row, opt, true)
}

func ExecuteSub2APIManagedRefreshPass() {
	if !sub2APIRefreshPassMu.TryLock() {
		return
	}
	defer sub2APIRefreshPassMu.Unlock()

	rows, err := db.ListActiveAccountsWithSiteByPlatform(sub2APIPlatform)
	if err != nil {
		slog.Warn("Sub2API managed refresh scan failed", "err", err)
		return
	}

	scanned := len(rows)
	refreshed := 0
	failed := 0
	for _, row := range rows {
		state := getSub2APIAuthState(row.ExtraConfig)
		if state.RefreshToken == "" || !isSub2APIManagedTokenDue(state.TokenExpiresAt, time.Now()) {
			continue
		}
		if _, _, didRefresh, err := ensureSub2APIManagedSession(row, requestOptionForAccount(row)); err != nil {
			failed++
			slog.Warn("Sub2API managed refresh failed", "account_id", row.ID, "err", err)
		} else if didRefresh {
			refreshed++
		}
	}

	if refreshed > 0 || failed > 0 {
		slog.Info("Sub2API managed refresh pass completed", "scanned", scanned, "refreshed", refreshed, "failed", failed)
	}
}

func StartSub2APIManagedRefreshScheduler() {
	sub2APIRefreshLoopMu.Lock()
	defer sub2APIRefreshLoopMu.Unlock()

	if sub2APIRefreshStopCh != nil {
		close(sub2APIRefreshStopCh)
	}
	stopCh := make(chan struct{})
	sub2APIRefreshStopCh = stopCh

	go func() {
		ExecuteSub2APIManagedRefreshPass()
		ticker := time.NewTicker(sub2APIManagedRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ExecuteSub2APIManagedRefreshPass()
			case <-stopCh:
				return
			}
		}
	}()
	slog.Info("Sub2API managed refresh scheduler started", "interval", sub2APIManagedRefreshInterval.String())
}

func StopSub2APIManagedRefreshScheduler() {
	sub2APIRefreshLoopMu.Lock()
	defer sub2APIRefreshLoopMu.Unlock()

	if sub2APIRefreshStopCh == nil {
		return
	}
	close(sub2APIRefreshStopCh)
	sub2APIRefreshStopCh = nil
	slog.Info("Sub2API managed refresh scheduler stopped")
}

func GetSub2APIManagedRefreshSchedulerStatus() (bool, int, int) {
	sub2APIRefreshLoopMu.Lock()
	defer sub2APIRefreshLoopMu.Unlock()

	return sub2APIRefreshStopCh != nil,
		int(sub2APIManagedRefreshInterval / time.Second),
		int(sub2APIManagedRefreshLead / time.Second)
}
