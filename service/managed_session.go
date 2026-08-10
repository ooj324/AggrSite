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
	managedRefreshLead     = 10 * time.Minute
	managedRefreshInterval = 5 * time.Minute
)

// managedSessionPlatforms are the platforms whose access token is short-lived and
// renewed in the background through Adapter.RefreshAuth.
var managedSessionPlatforms = []string{"sub2api", "new-api-v1"}

var (
	managedRefreshMu     sync.Map
	managedRefreshLoopMu sync.Mutex
	managedRefreshStopCh chan struct{}
	managedRefreshPassMu sync.Mutex
)

// ManagedSessionPlatforms returns the platforms covered by the managed refresh loop.
func ManagedSessionPlatforms() []string {
	return append([]string(nil), managedSessionPlatforms...)
}

// IsManagedSessionPlatform reports whether the platform uses background managed
// session refresh.
func IsManagedSessionPlatform(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, candidate := range managedSessionPlatforms {
		if lower == candidate {
			return true
		}
	}
	return false
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

// ApplyLoginManagedAuth persists managed-session credentials returned by a login
// into cfg, so the background refresher can keep the account alive afterwards.
// It reports whether cfg was modified.
func ApplyLoginManagedAuth(cfg map[string]interface{}, res *platform.LoginResult) bool {
	if cfg == nil || res == nil || strings.TrimSpace(res.RefreshCookie) == "" {
		return false
	}

	platform.SetNewApiV1RefreshCookie(cfg, res.RefreshCookie)

	expiresAt := platform.NormalizeEpochMillis(res.ExpiresAt)
	if expiresAt <= 0 {
		expiresAt = platform.JwtExpiresAtMillis(res.AccessToken)
	}
	if expiresAt <= 0 {
		// Neither reported nor derivable: assume a short TTL so the refresher runs
		// soon and replaces this guess with the real value from the refresh response.
		expiresAt = time.Now().Add(15 * time.Minute).UnixMilli()
	}
	platform.SetManagedTokenExpiresAt(cfg, expiresAt)
	return true
}

func parseTokenExpiresAt(raw interface{}) int64 {
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
	if value <= 0 {
		return 0
	}
	return platform.NormalizeEpochMillis(value)
}

// getManagedTokenExpiresAt extracts the access token expiry (unix millis) from
// ExtraConfig. The canonical location is managedAuth; the per-platform nodes are
// checked as a fallback for accounts written before the unification (and by the
// account form, which may still hold a legacy copy).
func getManagedTokenExpiresAt(extraConfig *string) int64 {
	if extraConfig == nil || *extraConfig == "" {
		return 0
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(*extraConfig), &cfg); err != nil {
		return 0
	}

	for _, key := range []string{
		platform.ManagedAuthConfigKey,
		platform.Sub2APIAuthConfigKey,
		platform.NewApiV1AuthConfigKey,
	} {
		node, ok := cfg[key].(map[string]interface{})
		if !ok || node == nil {
			continue
		}
		if expiresAt := parseTokenExpiresAt(node[platform.TokenExpiresAtKey]); expiresAt > 0 {
			return expiresAt
		}
	}
	return 0
}

// managedTokenExpiresAt resolves when the current credential dies: the recorded
// value first, otherwise the exp claim of the credential itself. Managed platforms
// issue JWT access tokens, so an account imported without an expiry still gets
// proactive refresh instead of waiting for the first failure.
func managedTokenExpiresAt(row db.AccountWithSite) int64 {
	if recorded := getManagedTokenExpiresAt(row.ExtraConfig); recorded > 0 {
		return recorded
	}
	return platform.JwtExpiresAtMillis(row.AccessToken)
}

func isManagedTokenDue(expiresAt int64, now time.Time) bool {
	if expiresAt <= 0 {
		return false
	}
	return expiresAt-now.UnixMilli() <= int64(managedRefreshLead/time.Millisecond)
}

func getManagedRefreshMutex(accountID int64) *sync.Mutex {
	mu, _ := managedRefreshMu.LoadOrStore(accountID, &sync.Mutex{})
	return mu.(*sync.Mutex)
}

// refreshManagedSession renews the account credential through Adapter.RefreshAuth.
// Without force it is a no-op until the token is within managedRefreshLead of
// expiring; with force it always attempts a refresh, which is what makes recovery
// possible for accounts whose expiry was never recorded.
func refreshManagedSession(row db.AccountWithSite, opt *platform.RequestOption, force bool) (string, string, bool, error) {
	if !IsManagedSessionPlatform(row.SitePlatform) {
		return row.AccessToken, stringValue(row.ExtraConfig), false, nil
	}

	if !force {
		if !isManagedTokenDue(managedTokenExpiresAt(row), time.Now()) {
			return row.AccessToken, stringValue(row.ExtraConfig), false, nil
		}
	}

	mu := getManagedRefreshMutex(row.ID)
	mu.Lock()
	defer mu.Unlock()

	latest, err := db.GetAccountWithSite(row.ID)
	if err != nil {
		return row.AccessToken, stringValue(row.ExtraConfig), false, err
	}
	latestExtraConfig := stringValue(latest.ExtraConfig)

	if force {
		// Another worker already rotated the credential while we waited.
		if latest.AccessToken != row.AccessToken {
			return latest.AccessToken, latestExtraConfig, false, nil
		}
	} else if !isManagedTokenDue(managedTokenExpiresAt(*latest), time.Now()) {
		return latest.AccessToken, latestExtraConfig, false, nil
	}

	adapter := platform.GetAdapter(latest.SitePlatform)
	if adapter == nil {
		return latest.AccessToken, latestExtraConfig, false, fmt.Errorf("unsupported managed platform: %s", latest.SitePlatform)
	}
	if opt == nil {
		opt = requestOptionForAccount(*latest)
	}

	res, err := adapter.RefreshAuth(latest.SiteURL, latest.AccessToken, latestExtraConfig, opt)
	if err != nil {
		return latest.AccessToken, latestExtraConfig, false, err
	}
	if res == nil || !res.Success || strings.TrimSpace(res.AccessToken) == "" {
		message := "managed session refresh failed"
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

	slog.Info("Managed session refreshed", "account_id", latest.ID, "platform", latest.SitePlatform)
	return res.AccessToken, res.ExtraConfig, true, nil
}

func EnsureManagedSession(row db.AccountWithSite, opt *platform.RequestOption) (string, string, bool, error) {
	return refreshManagedSession(row, opt, false)
}

func ForceRefreshManagedSession(row db.AccountWithSite, opt *platform.RequestOption) (string, string, bool, error) {
	return refreshManagedSession(row, opt, true)
}

func ExecuteManagedRefreshPass() {
	if !managedRefreshPassMu.TryLock() {
		return
	}
	defer managedRefreshPassMu.Unlock()

	rows, err := db.ListAccountsWithSiteByPlatforms(managedSessionPlatforms)
	if err != nil {
		slog.Warn("Managed refresh scan failed", "err", err)
		return
	}

	scanned := len(rows)
	refreshed := 0
	failed := 0
	for _, row := range rows {
		if !isManagedTokenDue(managedTokenExpiresAt(row), time.Now()) {
			continue
		}
		if _, _, didRefresh, err := EnsureManagedSession(row, requestOptionForAccount(row)); err != nil {
			failed++
			slog.Warn("Managed session refresh failed", "account_id", row.ID, "platform", row.SitePlatform, "err", err)
		} else if didRefresh {
			refreshed++
		}
	}

	if refreshed > 0 || failed > 0 {
		slog.Info("Managed session refresh pass completed", "scanned", scanned, "refreshed", refreshed, "failed", failed)
	}
}

func StartManagedRefreshScheduler() {
	managedRefreshLoopMu.Lock()
	defer managedRefreshLoopMu.Unlock()

	if managedRefreshStopCh != nil {
		close(managedRefreshStopCh)
	}
	stopCh := make(chan struct{})
	managedRefreshStopCh = stopCh

	go func() {
		ExecuteManagedRefreshPass()
		ticker := time.NewTicker(managedRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ExecuteManagedRefreshPass()
			case <-stopCh:
				return
			}
		}
	}()
	slog.Info("Managed session refresh scheduler started", "interval", managedRefreshInterval.String())
}

func StopManagedRefreshScheduler() {
	managedRefreshLoopMu.Lock()
	defer managedRefreshLoopMu.Unlock()

	if managedRefreshStopCh == nil {
		return
	}
	close(managedRefreshStopCh)
	managedRefreshStopCh = nil
	slog.Info("Managed session refresh scheduler stopped")
}

func GetManagedRefreshSchedulerStatus() (bool, int, int) {
	managedRefreshLoopMu.Lock()
	defer managedRefreshLoopMu.Unlock()

	return managedRefreshStopCh != nil,
		int(managedRefreshInterval / time.Second),
		int(managedRefreshLead / time.Second)
}
