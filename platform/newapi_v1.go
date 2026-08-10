package platform

import (
	"encoding/json"
	"fmt"
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
		return nil, fmt.Errorf("refresh request failed: %w", err)
	}

	if success, _ := res["success"].(bool); !success {
		message := ExtractMessage(res)
		if message == "" {
			message = "refresh failed"
		}
		return &RefreshResult{Success: false, Message: message}, nil
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

	// The refresh token rotates; keep the previous one if the response reuses it.
	newRefreshCookie := refreshCookie
	if cookieResult != nil && cookieResult.CookieHeader != "" {
		if value, ok := CookieValueFromHeader(cookieResult.CookieHeader, newApiV1RefreshCookieName); ok && strings.TrimSpace(value) != "" {
			newRefreshCookie = strings.TrimSpace(value)
		}
	}

	expiresAt := time.Now().Add(newApiV1FallbackTokenTTL).UnixMilli()
	if exp, ok := data["access_expires_at"].(float64); ok && exp > 0 {
		expiresAt = NormalizeEpochMillis(int64(exp))
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
