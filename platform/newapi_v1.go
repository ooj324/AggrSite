package platform

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// NewApiV1Adapter targets the QuantumNous new-api fork, whose panel auth is a
// short-lived access JWT plus an HttpOnly refresh cookie instead of a long-lived
// gin session cookie. Checkin/balance/token/model behaviour is identical to
// new-api, so it is inherited from NewApiAdapter; only the refresh flow differs.
type NewApiV1Adapter struct {
	NewApiAdapter
}

const (
	// newApiV1RefreshPath is guarded upstream by SessionCookieOriginGuard +
	// CriticalRateLimit + DisableCache, hence the Origin/Referer headers below.
	newApiV1RefreshPath = "/api/user/auth/refresh"
	// newApiV1RefreshCookieName is the cookie carrying the rotating refresh token.
	newApiV1RefreshCookieName = "new_api_refresh"
	// newApiV1FallbackTokenTTL is assumed when the upstream omits access_expires_at.
	// It is deliberately short: the next refresh reads the real expiry and self-corrects.
	newApiV1FallbackTokenTTL = 15 * time.Minute
	// newApiV1RateLimitBackoff matches the upstream CriticalRateLimit window
	// (20 requests / 20 minutes per client IP) that guards the refresh route.
	// Retrying inside the window only keeps the limiter saturated.
	newApiV1RateLimitBackoff = 20 * time.Minute
	// newApiV1RefreshRaceBackoff covers the upstream replay grace window, during
	// which a concurrent rotation makes the same refresh token usable again. The
	// retry must happen well inside that window; waiting for the full 30 seconds
	// risks turning a recoverable race into refresh-token reuse revocation.
	newApiV1RefreshRaceBackoff = 2 * time.Second
)

func init() {
	Register(&NewApiV1Adapter{NewApiAdapter{BaseAdapter: BaseAdapter{Name: "new-api-v1"}}})
}

// RefreshAuth exchanges the stored refresh cookie for a new access token via
// POST /api/user/auth/refresh. The refresh cookie rotates on every call, so the
// updated value is written back into extraConfig together with the new expiry.
//
// The rotating refresh cookie is a one-time credential. This method makes exactly
// one HTTP request; if the response is missing the rotated Set-Cookie or the
// request fails, the attempt is classified and returned to the managed-session
// scheduler for backoff-based retry. No automatic replay is attempted — replaying
// a possibly-consumed one-time cookie risks triggering upstream refresh-reuse
// detection (AUTH_SESSION_REVOKED), which permanently kills the session.
func (a *NewApiV1Adapter) RefreshAuth(baseURL, accessToken, extraConfig string, opt *RequestOption) (*RefreshResult, error) {
	cfg := map[string]interface{}{}
	if strings.TrimSpace(extraConfig) != "" {
		if err := json.Unmarshal([]byte(extraConfig), &cfg); err != nil {
			return &RefreshResult{Success: false, Message: "Invalid extraConfig format", CredentialDead: true}, nil
		}
		if cfg == nil {
			cfg = map[string]interface{}{}
		}
	}

	authNode, _ := cfg[NewApiV1AuthConfigKey].(map[string]interface{})
	refreshCookie := ""
	if authNode != nil {
		refreshCookie, _ = authNode[RefreshCookieKey].(string)
	}
	refreshCookie = strings.TrimSpace(refreshCookie)
	if refreshCookie == "" {
		// Accounts imported as a raw cookie jar still carry the refresh cookie.
		if value, ok := CookieValueFromHeader(stripBearerPrefix(accessToken), newApiV1RefreshCookieName); ok {
			refreshCookie = strings.TrimSpace(value)
		}
	}
	if refreshCookie == "" {
		return &RefreshResult{Success: false, Message: "No refresh cookie found; re-login required", CredentialDead: true}, nil
	}

	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	headers := map[string]string{
		"Cache-Control":    "no-store",
		"Origin":           base,
		"Referer":          base + "/",
		"X-Requested-With": "XMLHttpRequest",
	}

	var res map[string]interface{}
	cookieResult, err := fetchJSONOnce(
		base+newApiV1RefreshPath,
		"POST",
		newApiV1RefreshCookieName+"="+refreshCookie,
		headers,
		nil,
		&res,
		opt,
	)
	if err != nil {
		// Classify the failure so the scheduler knows whether waiting helps
		// (throttled) or whether the credential is gone (revoked/expired).
		return newApiV1RefreshFailure(err, res), nil
	}

	if success, _ := res["success"].(bool); !success {
		message := ExtractMessage(res)
		if message == "" {
			message = "refresh failed"
		}
		return newApiV1ClassifiedFailure(message, newApiV1ErrorCode(res), 0, 0), nil
	}

	data, _ := res["data"].(map[string]interface{})
	if data == nil {
		return &RefreshResult{Success: false, Message: "No data in successful refresh response; re-login required", CredentialDead: true}, nil
	}
	newAccessToken, _ := data["access_token"].(string)
	newAccessToken = strings.TrimSpace(newAccessToken)
	if newAccessToken == "" {
		return &RefreshResult{Success: false, Message: "No access_token in successful refresh response; re-login required", CredentialDead: true}, nil
	}

	// A successful upstream refresh always rotates this cookie. CookieHeader also
	// contains the value sent with the request, so only raw Set-Cookie response
	// values prove that the rotated secret reached us.
	newRefreshCookie, rotated := newApiV1RotatedRefreshCookie(cookieResult, refreshCookie)
	if !rotated {
		// The upstream 200 + success:true proves it consumed the old cookie and
		// rotated to a new value internally. Since Set-Cookie did not reach us
		// (proxy/CDN stripped it, etc.), the old cookie is dead. The scheduler's
		// minimum backoff (2 min) always exceeds the upstream's 30-second replay
		// grace window, so retrying with the consumed cookie is guaranteed to
		// trigger AUTH_SESSION_REVOKED. Mark the credential dead so autoRelogin
		// can attempt a password login to establish a fresh session.
		return &RefreshResult{
			Success:        false,
			Message:        "Upstream refreshed the access token but the rotated refresh cookie was not received; re-login required",
			CredentialDead: true,
		}, nil
	}

	// Expiry priority: what the panel reports, then the token's own exp claim, then a
	// deliberately short guess (the next refresh replaces it with a real value).
	expiresAt := int64(0)
	if exp, ok := data["access_expires_at"].(float64); ok && exp > 0 {
		expiresAt = NormalizeEpochMillis(int64(exp))
	}
	if expiresAt <= 0 {
		expiresAt = JwtExpiresAtMillis(newAccessToken)
	}
	if expiresAt <= 0 {
		expiresAt = time.Now().Add(newApiV1FallbackTokenTTL).UnixMilli()
	}

	if authNode == nil {
		authNode = map[string]interface{}{}
	}
	authNode[RefreshCookieKey] = newRefreshCookie
	cfg[NewApiV1AuthConfigKey] = authNode
	SetManagedTokenExpiresAt(cfg, expiresAt)

	newExtraConfig, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal new extraConfig: %w", err)
	}

	return &RefreshResult{
		Success:     true,
		AccessToken: newAccessToken,
		ExtraConfig: string(newExtraConfig),
		Message:     "Refreshed via new-api refresh cookie",
	}, nil
}

func newApiV1RotatedRefreshCookie(result *FetchCookieResult, previous string) (string, bool) {
	if result == nil {
		return "", false
	}
	var latest string
	found := false
	for _, raw := range result.SetCookies {
		if value, ok := CookieValueFromHeader(raw, newApiV1RefreshCookieName); ok {
			latest = strings.TrimSpace(value)
			found = true
		}
	}
	return latest, found && latest != "" && latest != strings.TrimSpace(previous)
}

// newApiV1RefreshFailure turns a transport/HTTP failure into a classified result.
// Upstream answers refresh errors with {"success":false,"code":"AUTH_..."} and a
// generic status text, so the code - not the message - carries the meaning.
func newApiV1RefreshFailure(err error, res map[string]interface{}) *RefreshResult {
	code := newApiV1ErrorCode(res)
	message := "refresh request failed: " + err.Error()
	if code != "" {
		message += " (" + code + ")"
	}
	return newApiV1ClassifiedFailure(message, code, HTTPStatusFromError(err), RetryAfterFromError(err))
}

func newApiV1ClassifiedFailure(message, code string, status int, retryHint time.Duration) *RefreshResult {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code != "" && !strings.Contains(strings.ToUpper(message), code) {
		message += " (" + code + ")"
	}
	result := &RefreshResult{Success: false, Message: message}

	// Structured upstream codes are authoritative. In particular, 403 can mean
	// an Origin/reverse-proxy configuration error rather than a dead credential,
	// while 409 can mean either a recoverable race or a fatal session mismatch.
	switch {
	case code == "AUTH_REFRESH_RACE":
		result.RetryAfter = newApiV1RefreshRaceBackoff
		return result
	case newApiV1CodeIsFatal(code):
		result.CredentialDead = true
		return result
	case code == "AUTH_ORIGIN_FORBIDDEN":
		return result
	}

	switch {
	case status == http.StatusTooManyRequests:
		result.RetryAfter = retryHint
		if result.RetryAfter <= 0 {
			result.RetryAfter = newApiV1RateLimitBackoff
		}
	case status == http.StatusConflict:
		// Limited fallback for older upstreams that did not return an AUTH_* code.
		result.RetryAfter = newApiV1RefreshRaceBackoff
	case status == http.StatusUnauthorized:
		result.CredentialDead = true
	case status == http.StatusForbidden:
		// Refresh-cookie rejection is reported as 401 upstream. An unstructured
		// 403 is more likely an origin guard, WAF, or reverse-proxy policy and is
		// not proof that password login is required.
	}
	return result
}

func newApiV1ErrorCode(res map[string]interface{}) string {
	if res == nil {
		return ""
	}
	code, _ := res["code"].(string)
	return strings.TrimSpace(code)
}

// newApiV1CodeIsFatal reports codes that mean the refresh cookie will never work
// again: the session was revoked (including upstream refresh-reuse detection) or
// the presented token is not recognised at all.
func newApiV1CodeIsFatal(code string) bool {
	switch strings.ToUpper(code) {
	case "AUTH_SESSION_REVOKED", "AUTH_UNAUTHORIZED", "AUTH_TOKEN_EXPIRED", "AUTH_SESSION_MISMATCH", "AUTH_USER_DISABLED":
		return true
	}
	return false
}
