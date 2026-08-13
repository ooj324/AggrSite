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
	// managedRefreshInterval is how often the scheduler scans for due credentials.
	// The scan itself only reads the database; upstream calls happen when due.
	managedRefreshInterval = 2 * time.Minute
	// managedRefreshLead is the minimum time before expiry at which a credential is
	// renewed. It must stay comfortably above managedRefreshInterval, otherwise a
	// credential can die between two scans.
	managedRefreshLead = 4 * time.Minute
	// managedRefreshMaxLead caps how early a long-lived credential is renewed.
	managedRefreshMaxLead = 10 * time.Minute
	// managedRefreshLeadDivisor renews a credential in the last 1/N of its own
	// lifetime. new-api access tokens live 15 minutes, so the previous fixed
	// 10-minute lead rotated them every ~5 minutes and exhausted the upstream
	// CriticalRateLimit budget (20 requests / 20 minutes per client IP) that guards
	// POST /api/user/auth/refresh - after which no account could refresh at all.
	managedRefreshLeadDivisor = 4
	// managedRefreshMinSpacing collapses back-to-back refreshes of one account
	// (proactive scan followed by a reactive force refresh). Every refresh rotates
	// the upstream refresh token, so needless rotations are pure risk.
	managedRefreshMinSpacing = 60 * time.Second
	// Failure backoff: without it a throttled or revoked session is retried on every
	// scan, which keeps the upstream limiter saturated and never recovers.
	managedRefreshBackoffBase = 2 * time.Minute
	managedRefreshBackoffMax  = 30 * time.Minute
	// managedRefreshDeadBackoff applies when the refresh credential itself is gone.
	// Recovery needs a new login, so probing it more often is wasted budget.
	managedRefreshDeadBackoff = 6 * time.Hour
)

// managedSessionPlatforms are the platforms whose access token is short-lived and
// renewed in the background through Adapter.RefreshAuth.
var managedSessionPlatforms = []string{"sub2api", "new-api-v1"}

var (
	managedRefreshStates sync.Map
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
	return isManagedTokenDueWithLead(expiresAt, now, managedRefreshLead)
}

func isManagedTokenDueWithLead(expiresAt int64, now time.Time, lead time.Duration) bool {
	if expiresAt <= 0 {
		return false
	}
	return expiresAt-now.UnixMilli() <= int64(lead/time.Millisecond)
}

// managedRefreshLeadFor scales the renewal lead to the credential's own lifetime,
// so a 15-minute new-api access token is rotated roughly once per lifetime instead
// of on every scan, while a long-lived credential still gets a wide margin.
func managedRefreshLeadFor(accessToken string) time.Duration {
	lifetime := platform.JwtLifetimeMillis(accessToken)
	if lifetime <= 0 {
		return managedRefreshLead
	}
	lead := time.Duration(lifetime/managedRefreshLeadDivisor) * time.Millisecond
	if lead < managedRefreshLead {
		return managedRefreshLead
	}
	if lead > managedRefreshMaxLead {
		return managedRefreshMaxLead
	}
	return lead
}

func isManagedRefreshDue(row db.AccountWithSite, now time.Time) bool {
	return isManagedTokenDueWithLead(managedTokenExpiresAt(row), now, managedRefreshLeadFor(row.AccessToken))
}

// managedRefreshState serialises refreshes of one account and remembers the outcome,
// which is what turns a failing credential into a backing-off retry instead of a
// request on every scan.
type managedRefreshState struct {
	mu          sync.Mutex
	credential  string // access token the remembered outcome belongs to
	lastSuccess time.Time
	nextAttempt time.Time
	failures    int
}

func getManagedRefreshState(accountID int64) *managedRefreshState {
	state, _ := managedRefreshStates.LoadOrStore(accountID, &managedRefreshState{})
	return state.(*managedRefreshState)
}

// syncCredential drops the remembered outcome when the stored credential changed
// behind our back (relogin, rebind, manual edit): a new credential starts with a
// clean slate instead of inheriting the previous one's backoff.
func (s *managedRefreshState) syncCredential(accessToken string) {
	if s.credential == accessToken {
		return
	}
	s.credential = accessToken
	s.lastSuccess = time.Time{}
	s.nextAttempt = time.Time{}
	s.failures = 0
}

// throttled reports whether a refresh attempt must be skipped right now. Callers
// hold state.mu.
func (s *managedRefreshState) throttled(now time.Time) bool {
	if !s.lastSuccess.IsZero() && now.Sub(s.lastSuccess) < managedRefreshMinSpacing {
		return true
	}
	return !s.nextAttempt.IsZero() && now.Before(s.nextAttempt)
}

func (s *managedRefreshState) recordSuccess(now time.Time, accessToken string) {
	s.credential = accessToken
	s.lastSuccess = now
	s.nextAttempt = time.Time{}
	s.failures = 0
}

func (s *managedRefreshState) recordFailure(now time.Time, res *platform.RefreshResult) time.Duration {
	s.failures++
	backoff := managedRefreshBackoff(s.failures, res)
	s.nextAttempt = now.Add(backoff)
	return backoff
}

func managedRefreshBackoff(failures int, res *platform.RefreshResult) time.Duration {
	if res != nil {
		if res.CredentialDead {
			return managedRefreshDeadBackoff
		}
		if res.RetryAfter > 0 {
			return res.RetryAfter
		}
	}
	if failures < 1 {
		failures = 1
	}
	backoff := managedRefreshBackoffBase
	for i := 1; i < failures && backoff < managedRefreshBackoffMax; i++ {
		backoff *= 2
	}
	if backoff > managedRefreshBackoffMax {
		backoff = managedRefreshBackoffMax
	}
	return backoff
}

// refreshManagedSession renews the account credential through Adapter.RefreshAuth.
// Without force it is a no-op until the token enters its renewal window; with force
// it always attempts a refresh, which is what makes recovery possible for accounts
// whose expiry was never recorded. Both paths respect the per-account backoff: the
// upstream refresh route is rate limited and rotates its refresh token on every
// call, so retrying blindly is what kills a session rather than saving it.
func refreshManagedSession(row db.AccountWithSite, opt *platform.RequestOption, force bool) (string, string, bool, error) {
	if !IsManagedSessionPlatform(row.SitePlatform) {
		return row.AccessToken, stringValue(row.ExtraConfig), false, nil
	}

	if !force {
		if !isManagedRefreshDue(row, time.Now()) {
			return row.AccessToken, stringValue(row.ExtraConfig), false, nil
		}
	}

	state := getManagedRefreshState(row.ID)
	state.mu.Lock()
	defer state.mu.Unlock()

	latest, err := db.GetAccountWithSite(row.ID)
	if err != nil {
		return row.AccessToken, stringValue(row.ExtraConfig), false, err
	}
	latestExtraConfig := stringValue(latest.ExtraConfig)

	now := time.Now()
	state.syncCredential(latest.AccessToken)
	if state.throttled(now) {
		// Either another caller just rotated the credential (so this row was simply
		// stale) or the last attempt failed and we are waiting out its backoff. The
		// failure was already logged when it happened.
		return latest.AccessToken, latestExtraConfig, false, nil
	}

	if force {
		// Another worker already rotated the credential while we waited.
		if latest.AccessToken != row.AccessToken {
			return latest.AccessToken, latestExtraConfig, false, nil
		}
	} else if !isManagedRefreshDue(*latest, now) {
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
		backoff := state.recordFailure(time.Now(), res)
		return latest.AccessToken, latestExtraConfig, false, fmt.Errorf("%w (retrying in %s)", err, backoff)
	}
	if res == nil || !res.Success || strings.TrimSpace(res.AccessToken) == "" {
		backoff := state.recordFailure(time.Now(), res)
		message := "managed session refresh failed"
		if res != nil && strings.TrimSpace(res.Message) != "" {
			message = res.Message
		}
		if res != nil && res.CredentialDead {
			message += "; the refresh credential is gone, a new login is required"
			// Surface it: the account keeps failing until someone re-logs in, and the
			// backoff means this is logged at most once per window.
			_ = db.InsertEvent("account", "托管会话续期凭据失效",
				fmt.Sprintf("%s @ %s: %s", nullStr(latest.Username), latest.SiteName, message),
				"error", &latest.ID, "account")
		}
		return latest.AccessToken, latestExtraConfig, false, fmt.Errorf("%s (retrying in %s)", message, backoff)
	}

	updates := map[string]interface{}{
		"access_token": res.AccessToken,
		"extra_config": res.ExtraConfig,
	}
	if latest.Status != nil && *latest.Status == "expired" {
		updates["status"] = "active"
	}
	if err := db.UpdateAccount(latest.ID, updates); err != nil {
		// The upstream refresh token has already rotated at this point. Retry once
		// immediately: losing this write means the stored credential is stale and the
		// next refresh would be rejected as a replay.
		if retryErr := db.UpdateAccount(latest.ID, updates); retryErr != nil {
			slog.Error("Managed session refreshed but persisting the rotated credential failed",
				"account_id", latest.ID, "platform", latest.SitePlatform, "err", retryErr)
			state.recordFailure(time.Now(), nil)
			return latest.AccessToken, latestExtraConfig, false, retryErr
		}
	}

	state.recordSuccess(time.Now(), res.AccessToken)
	slog.Info("Managed session refreshed", "account_id", latest.ID, "platform", latest.SitePlatform, "detail", res.Message)
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
		if !isManagedRefreshDue(row, time.Now()) {
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
