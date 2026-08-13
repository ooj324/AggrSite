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
	// which a concurrent rotation makes the same refresh token usable again.
	newApiV1RefreshRaceBackoff = 30 * time.Second
)

func init() {
	Register(&NewApiV1Adapter{NewApiAdapter{BaseAdapter: BaseAdapter{Name: "new-api-v1"}}})
}

// RefreshAuth exchanges the stored refresh cookie for a new access token via
// POST /api/user/auth/refresh. The refresh cookie rotates on every call, so the
// updated value is written back into extraConfig together with the new expiry.
func (a *NewApiV1Adapter) RefreshAuth(baseURL, accessToken, extraConfig string, opt *RequestOption) (*RefreshResult, error) {
	cfg := map[string]interface{}{}
	if strings.TrimSpace(extraConfig) != "" {
		if err := json.Unmarshal([]byte(extraConfig), &cfg); err != nil {
			return &RefreshResult{Success: false, Message: "Invalid extraConfig format"}, nil
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
		return &RefreshResult{Success: false, Message: "No refresh cookie found; re-login required"}, nil
	}

	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	headers := map[string]string{
		"Cache-Control":    "no-store",
		"Origin":           base,
		"Referer":          base + "/",
		"X-Requested-With": "XMLHttpRequest",
	}

	var res map[string]interface{}
	cookieResult, err := FetchJSONWithCookieRetry(
		base+newApiV1RefreshPath,
		"POST",
		newApiV1RefreshCookieName+"="+refreshCookie,
		headers,
		nil,
		&res,
		opt,
	)
	if err != nil {
		// Classified instead of returned as a bare error: the scheduler has to know
		// whether waiting helps (throttled) or whether the credential is gone.
		return newApiV1RefreshFailure(err, res), nil
	}

	if success, _ := res["success"].(bool); !success {
		message := ExtractMessage(res)
		if message == "" {
			message = "refresh failed"
		}
		result := &RefreshResult{Success: false, Message: message}
		if code := newApiV1ErrorCode(res); code != "" {
			result.Message = message + " (" + code + ")"
			result.CredentialDead = newApiV1CodeIsFatal(code)
		}
		return result, nil
	}

	data, _ := res["data"].(map[string]interface{})
	if data == nil {
		return &RefreshResult{Success: false, Message: "No data in refresh response"}, nil
	}
	newAccessToken, _ := data["access_token"].(string)
	newAccessToken = strings.TrimSpace(newAccessToken)
	if newAccessToken == "" {
		return &RefreshResult{Success: false, Message: "No access_token in refresh response"}, nil
	}

	// The refresh token rotates on every successful refresh; keep the previous value
	// only if the response did not carry a new one.
	newRefreshCookie := refreshCookie
	if cookieResult != nil && cookieResult.CookieHeader != "" {
		if value, ok := CookieValueFromHeader(cookieResult.CookieHeader, newApiV1RefreshCookieName); ok && strings.TrimSpace(value) != "" {
			newRefreshCookie = strings.TrimSpace(value)
		}
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

	message := "Refreshed via new-api refresh cookie"
	if newRefreshCookie == refreshCookie {
		// Upstream always rotates the refresh token when it answers 200, so an unchanged
		// cookie means the rotated value never reached us (stripped in transit). The
		// stored one is now the previous secret and will be rejected as a replay.
		message += " (no rotated refresh cookie in the response; the next refresh will likely need a new login)"
	}

	return &RefreshResult{
		Success:     true,
		AccessToken: newAccessToken,
		ExtraConfig: string(newExtraConfig),
		Message:     message,
	}, nil
}

// newApiV1RefreshFailure turns a transport/HTTP failure into a classified result.
// Upstream answers refresh errors with {"success":false,"code":"AUTH_..."} and a
// generic status text, so the code - not the message - carries the meaning.
func newApiV1RefreshFailure(err error, res map[string]interface{}) *RefreshResult {
	result := &RefreshResult{Success: false, Message: "refresh request failed: " + err.Error()}
	code := newApiV1ErrorCode(res)
	if code != "" {
		result.Message += " (" + code + ")"
	}

	switch status := HTTPStatusFromError(err); {
	case status == http.StatusTooManyRequests:
		result.RetryAfter = newApiV1RateLimitBackoff
	case status == http.StatusConflict, code == "AUTH_REFRESH_RACE":
		// A parallel refresh already rotated the token; inside the upstream replay
		// window the very same cookie still works, so retry soon rather than giving up.
		result.RetryAfter = newApiV1RefreshRaceBackoff
	case status == http.StatusUnauthorized, status == http.StatusForbidden, newApiV1CodeIsFatal(code):
		result.CredentialDead = true
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
