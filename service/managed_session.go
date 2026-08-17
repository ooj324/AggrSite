package service

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"metapi/aggrsite/db"
	"metapi/aggrsite/platform"
	"sort"
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
	// Both upstream refresh routes are IP limited. Pacing by site and outbound route
	// prevents accounts that expire together from spending the whole shared window
	// in one burst (new-api: 20 / 20m; Sub2API: 30 / minute).
	managedNewAPISiteSpacing = time.Minute
	managedSub2SiteSpacing   = 2 * time.Second
)

// managedSessionPlatforms are the platforms whose access token is short-lived and
// renewed in the background through Adapter.RefreshAuth.
var managedSessionPlatforms = []string{"sub2api", "new-api-v1"}

var (
	managedRefreshStates       sync.Map
	managedRefreshSiteStates   sync.Map
	managedRefreshLoopMu       sync.Mutex
	managedRefreshStopCh       chan struct{}
	managedRefreshWakeTimer    *time.Timer
	managedRefreshWakeAt       time.Time
	managedRefreshPassMu       sync.Mutex
	errManagedRefreshDeferred  = errors.New("managed session refresh deferred")
	errManagedRefreshRetryable = errors.New("managed session refresh failed temporarily")
	errManagedCredentialDead   = errors.New("managed session refresh credential is unusable")
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

// managedTokenExpiresAt resolves when the current credential dies. A JWT's exp is
// bound to the access token itself and therefore wins over a recorded value that
// may belong to a token replaced manually. Opaque credentials still use the
// recorded expiry as their only available hint.
func managedTokenExpiresAt(row db.AccountWithSite) int64 {
	if tokenExpiry := platform.JwtExpiresAtMillis(row.AccessToken); tokenExpiry > 0 {
		return tokenExpiry
	}
	return getManagedTokenExpiresAt(row.ExtraConfig)
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
	mu             sync.Mutex
	credential     string // fingerprint of access + refresh credentials and route
	lastSuccess    time.Time
	nextAttempt    time.Time
	failures       int
	credentialDead bool
}

func getManagedRefreshState(accountID int64) *managedRefreshState {
	state, _ := managedRefreshStates.LoadOrStore(accountID, &managedRefreshState{})
	return state.(*managedRefreshState)
}

// syncCredential drops the remembered outcome when the stored credential changed
// behind our back (relogin, rebind, manual edit): a new credential starts with a
// clean slate instead of inheriting the previous one's backoff.
func (s *managedRefreshState) syncCredential(fingerprint string) {
	if s.credential == fingerprint {
		return
	}
	s.credential = fingerprint
	s.lastSuccess = time.Time{}
	s.nextAttempt = time.Time{}
	s.failures = 0
	s.credentialDead = false
}

// throttled reports whether a refresh attempt must be skipped right now. Callers
// hold state.mu.
func (s *managedRefreshState) throttled(now time.Time) bool {
	return s.retryDelay(now) > 0
}

func (s *managedRefreshState) retryDelay(now time.Time) time.Duration {
	var wait time.Duration
	if !s.lastSuccess.IsZero() {
		if remaining := managedRefreshMinSpacing - now.Sub(s.lastSuccess); remaining > wait {
			wait = remaining
		}
	}
	if !s.nextAttempt.IsZero() {
		if remaining := s.nextAttempt.Sub(now); remaining > wait {
			wait = remaining
		}
	}
	return wait
}

func (s *managedRefreshState) recordSuccess(now time.Time, fingerprint string) {
	s.credential = fingerprint
	s.lastSuccess = now
	s.nextAttempt = time.Time{}
	s.failures = 0
	s.credentialDead = false
}

func (s *managedRefreshState) recordFailure(now time.Time, res *platform.RefreshResult) time.Duration {
	s.failures++
	s.credentialDead = res != nil && res.CredentialDead
	backoff := managedRefreshBackoff(s.failures, res)
	s.nextAttempt = now.Add(backoff)
	return backoff
}

type managedRefreshWaitError struct {
	kind       error
	detail     string
	retryAfter time.Duration
}

func (e *managedRefreshWaitError) Error() string {
	detail := strings.TrimSpace(e.detail)
	if detail == "" && e.kind != nil {
		detail = e.kind.Error()
	}
	if e.retryAfter <= 0 {
		return detail
	}
	wait := e.retryAfter.Round(time.Second)
	if wait <= 0 {
		wait = time.Second
	}
	return fmt.Sprintf("%s (retrying in %s)", detail, wait)
}

func (e *managedRefreshWaitError) Unwrap() error {
	return e.kind
}

func newManagedRefreshWaitError(kind error, detail string, retryAfter time.Duration) error {
	return &managedRefreshWaitError{kind: kind, detail: detail, retryAfter: retryAfter}
}

func managedRefreshRetryDelay(err error) time.Duration {
	var waitErr *managedRefreshWaitError
	if errors.As(err, &waitErr) {
		return waitErr.retryAfter
	}
	return 0
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

func managedRefreshCredential(row db.AccountWithSite, cfg map[string]interface{}) string {
	platformName := strings.ToLower(strings.TrimSpace(row.SitePlatform))
	var nodeKey, valueKey string
	switch platformName {
	case "sub2api":
		nodeKey, valueKey = platform.Sub2APIAuthConfigKey, platform.RefreshTokenKey
	case "new-api-v1":
		nodeKey, valueKey = platform.NewApiV1AuthConfigKey, platform.RefreshCookieKey
	default:
		return ""
	}

	if cfg != nil {
		if rawNode, exists := cfg[nodeKey]; exists {
			if node, ok := rawNode.(map[string]interface{}); ok && node != nil {
				if rawValue, exists := node[valueKey]; exists {
					if value, ok := rawValue.(string); ok {
						return strings.TrimSpace(value)
					}
					if encoded, err := json.Marshal(rawValue); err == nil {
						return "invalid:" + string(encoded)
					}
				}
			} else if encoded, err := json.Marshal(rawNode); err == nil {
				// Preserve malformed relevant nodes in the fingerprint so repairing the
				// configuration immediately clears an old backoff.
				return "invalid:" + string(encoded)
			}
		}
	}

	if platformName == "new-api-v1" {
		if value, ok := platform.CookieValueFromHeader(row.AccessToken, "new_api_refresh"); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func managedCredentialFingerprint(row db.AccountWithSite) string {
	var cfg map[string]interface{}
	extraConfig := stringValue(row.ExtraConfig)
	invalidConfig := ""
	if strings.TrimSpace(extraConfig) != "" {
		if err := json.Unmarshal([]byte(extraConfig), &cfg); err != nil {
			invalidConfig = extraConfig
		}
	}

	proxyURL := stringValue(row.SiteProxyURL)
	useSystemProxy := "unset"
	autoRelogin := ""
	if row.SiteUseSystemProxy != nil {
		useSystemProxy = strconv.FormatBool(*row.SiteUseSystemProxy)
	}
	if cfg != nil {
		if value, ok := cfg["proxyUrl"].(string); ok && strings.TrimSpace(value) != "" {
			proxyURL = strings.TrimSpace(value)
		}
		if value, ok := cfg["useSystemProxy"].(bool); ok {
			useSystemProxy = strconv.FormatBool(value)
		}
		if value, exists := cfg["autoRelogin"]; exists {
			if encoded, err := json.Marshal(value); err == nil {
				autoRelogin = string(encoded)
			}
		}
	}

	parts := []string{
		strconv.FormatInt(row.SiteID, 10),
		stringValue(row.CreatedAt),
		strings.ToLower(strings.TrimSpace(row.SitePlatform)),
		strings.TrimRight(strings.TrimSpace(row.SiteURL), "/"),
		row.AccessToken,
		managedRefreshCredential(row, cfg),
		proxyURL,
		useSystemProxy,
		managedRefreshRoute(requestOptionForAccount(row)),
		stringValue(row.SiteCustomHeaders),
		stringValue(row.SiteTurnstileSiteKey),
		autoRelogin,
		invalidConfig,
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return fmt.Sprintf("%x", sum[:])
}

func managedRefreshSiteSpacing(platformName string) time.Duration {
	switch strings.ToLower(strings.TrimSpace(platformName)) {
	case "new-api-v1":
		return managedNewAPISiteSpacing
	case "sub2api":
		return managedSub2SiteSpacing
	default:
		return 0
	}
}

func managedRefreshRoute(opt *platform.RequestOption) string {
	return platform.RequestRouteFingerprint(opt)
}

func managedRefreshSiteScope(row db.AccountWithSite, opt *platform.RequestOption) string {
	route := managedRefreshRoute(opt)
	raw := strings.ToLower(strings.TrimSpace(row.SitePlatform)) + "\x00" +
		strings.TrimRight(strings.TrimSpace(row.SiteURL), "/") + "\x00" + route
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum[:])
}

type managedRefreshSiteState struct {
	mu          sync.Mutex
	inFlight    bool
	nextAttempt time.Time
}

func getManagedRefreshSiteState(scope string) *managedRefreshSiteState {
	state, _ := managedRefreshSiteStates.LoadOrStore(scope, &managedRefreshSiteState{})
	return state.(*managedRefreshSiteState)
}

func (s *managedRefreshSiteState) tryStart(now time.Time, spacing time.Duration) (bool, time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inFlight {
		return false, spacing
	}
	if !s.nextAttempt.IsZero() && now.Before(s.nextAttempt) {
		return false, s.nextAttempt.Sub(now)
	}
	s.inFlight = true
	return true, 0
}

func (s *managedRefreshSiteState) finish(now time.Time, spacing time.Duration, res *platform.RefreshResult, err error) {
	wait := spacing
	if res != nil && res.RetryAfter > 0 {
		wait = res.RetryAfter
	} else if retryAfter := platform.RetryAfterFromError(err); retryAfter > 0 {
		wait = retryAfter
	}

	s.mu.Lock()
	s.inFlight = false
	if wait > 0 {
		s.nextAttempt = now.Add(wait)
	} else {
		s.nextAttempt = time.Time{}
	}
	s.mu.Unlock()
}

func isManagedRefreshDeferred(err error) bool {
	return errors.Is(err, errManagedRefreshDeferred)
}

func isManagedRefreshCredentialDead(err error) bool {
	return errors.Is(err, errManagedCredentialDead)
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
	state.syncCredential(managedCredentialFingerprint(*latest))
	if force && latest.AccessToken != row.AccessToken {
		// The caller held a stale row while another worker already persisted a new
		// credential. Return that credential before consulting the old attempt's
		// spacing; this is a successful hand-off, not a deferred refresh.
		return latest.AccessToken, latestExtraConfig, false, nil
	}
	if wait := state.retryDelay(now); wait > 0 {
		// Either another caller just rotated the credential (so this row was simply
		// stale) or the last attempt failed and we are waiting out its backoff. The
		// failure was already logged when it happened.
		if force {
			if state.credentialDead {
				return latest.AccessToken, latestExtraConfig, false,
					newManagedRefreshWaitError(errManagedCredentialDead, errManagedCredentialDead.Error(), wait)
			}
			return latest.AccessToken, latestExtraConfig, false,
				newManagedRefreshWaitError(errManagedRefreshDeferred, errManagedRefreshDeferred.Error(), wait)
		}
		return latest.AccessToken, latestExtraConfig, false, nil
	}

	if !force && !isManagedRefreshDue(*latest, now) {
		return latest.AccessToken, latestExtraConfig, false, nil
	}

	adapter := platform.GetAdapter(latest.SitePlatform)
	if adapter == nil {
		return latest.AccessToken, latestExtraConfig, false, fmt.Errorf("unsupported managed platform: %s", latest.SitePlatform)
	}
	if opt == nil {
		opt = requestOptionForAccount(*latest)
	}

	spacing := managedRefreshSiteSpacing(latest.SitePlatform)
	var siteState *managedRefreshSiteState
	if spacing > 0 {
		siteState = getManagedRefreshSiteState(managedRefreshSiteScope(*latest, opt))
		started, wait := siteState.tryStart(now, spacing)
		if !started {
			if wait <= 0 {
				wait = spacing
			}
			return latest.AccessToken, latestExtraConfig, false,
				newManagedRefreshWaitError(errManagedRefreshDeferred, errManagedRefreshDeferred.Error(), wait)
		}
	}

	var res *platform.RefreshResult
	var refreshErr error
	if siteState != nil {
		defer func() {
			siteState.finish(time.Now(), spacing, res, refreshErr)
		}()
	}

	res, refreshErr = adapter.RefreshAuth(latest.SiteURL, latest.AccessToken, latestExtraConfig, opt)
	if refreshErr != nil {
		backoff := state.recordFailure(time.Now(), res)
		return latest.AccessToken, latestExtraConfig, false,
			newManagedRefreshWaitError(errManagedRefreshRetryable, refreshErr.Error(), backoff)
	}
	if res != nil && res.Success && strings.TrimSpace(res.ExtraConfig) == "" {
		res = &platform.RefreshResult{
			Success: false,
			Message: "managed session refresh returned no updated extraConfig",
		}
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
		failureKind := errManagedRefreshRetryable
		if res != nil && res.CredentialDead {
			failureKind = errManagedCredentialDead
		}
		return latest.AccessToken, latestExtraConfig, false,
			newManagedRefreshWaitError(failureKind, message, backoff)
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
			lostCredential := &platform.RefreshResult{CredentialDead: true}
			backoff := state.recordFailure(time.Now(), lostCredential)
			message := "upstream rotated the refresh credential, but the replacement could not be persisted; re-login or rebind is required"
			_ = db.InsertEvent("account", "托管会话续期凭据保存失败",
				fmt.Sprintf("%s @ %s: %s", nullStr(latest.Username), latest.SiteName, message),
				"error", &latest.ID, "account")
			return latest.AccessToken, latestExtraConfig, false,
				newManagedRefreshWaitError(errManagedCredentialDead, message+": "+retryErr.Error(), backoff)
		}
	}

	updated := *latest
	updated.AccessToken = res.AccessToken
	updated.ExtraConfig = &res.ExtraConfig
	state.recordSuccess(time.Now(), managedCredentialFingerprint(updated))
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
	sort.SliceStable(rows, func(i, j int) bool {
		left := managedTokenExpiresAt(rows[i])
		right := managedTokenExpiresAt(rows[j])
		switch {
		case left <= 0:
			return false
		case right <= 0:
			return true
		default:
			return left < right
		}
	})

	scanned := len(rows)
	refreshed := 0
	failed := 0
	var nextWake time.Duration
	for _, row := range rows {
		if !isManagedRefreshDue(row, time.Now()) {
			continue
		}
		if _, _, didRefresh, err := EnsureManagedSession(row, requestOptionForAccount(row)); err != nil {
			if delay := managedRefreshRetryDelay(err); delay > 0 && (nextWake <= 0 || delay < nextWake) {
				nextWake = delay
			}
			if isManagedRefreshDeferred(err) {
				continue
			}
			failed++
			slog.Warn("Managed session refresh failed", "account_id", row.ID, "platform", row.SitePlatform, "err", err)
		} else if didRefresh {
			refreshed++
		}
	}
	if nextWake > 0 {
		scheduleManagedRefreshWake(nextWake)
	}

	if refreshed > 0 || failed > 0 {
		slog.Info("Managed session refresh pass completed", "scanned", scanned, "refreshed", refreshed, "failed", failed)
	}
}

func scheduleManagedRefreshWake(delay time.Duration) {
	if delay <= 0 {
		return
	}
	deadline := time.Now().Add(delay)

	managedRefreshLoopMu.Lock()
	defer managedRefreshLoopMu.Unlock()
	if managedRefreshStopCh == nil {
		return
	}
	if managedRefreshWakeTimer != nil && !deadline.Before(managedRefreshWakeAt) {
		return
	}
	if managedRefreshWakeTimer != nil {
		managedRefreshWakeTimer.Stop()
	}

	stopCh := managedRefreshStopCh
	managedRefreshWakeAt = deadline
	managedRefreshWakeTimer = time.AfterFunc(time.Until(deadline), func() {
		managedRefreshLoopMu.Lock()
		if managedRefreshStopCh != stopCh {
			managedRefreshLoopMu.Unlock()
			return
		}
		managedRefreshWakeTimer = nil
		managedRefreshWakeAt = time.Time{}
		managedRefreshLoopMu.Unlock()

		ExecuteManagedRefreshPass()
	})
}

func stopManagedRefreshWakeLocked() {
	if managedRefreshWakeTimer != nil {
		managedRefreshWakeTimer.Stop()
		managedRefreshWakeTimer = nil
	}
	managedRefreshWakeAt = time.Time{}
}

func StartManagedRefreshScheduler() {
	managedRefreshLoopMu.Lock()
	defer managedRefreshLoopMu.Unlock()

	stopManagedRefreshWakeLocked()
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

	stopManagedRefreshWakeLocked()
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
