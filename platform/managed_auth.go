package platform

import "strings"

// Config node names used to persist managed-session state inside an account's
// extra_config. They live in one place so writers (adapters, handlers) and
// readers (the managed refresh scheduler) cannot drift apart.
const (
	// ManagedAuthConfigKey is the canonical, platform-agnostic session node.
	ManagedAuthConfigKey = "managedAuth"
	// Sub2APIAuthConfigKey holds sub2api refresh credentials.
	Sub2APIAuthConfigKey = "sub2apiAuth"
	// NewApiV1AuthConfigKey holds QuantumNous new-api refresh credentials.
	NewApiV1AuthConfigKey = "newApiV1Auth"

	TokenExpiresAtKey = "tokenExpiresAt"
	RefreshTokenKey   = "refreshToken"
	RefreshCookieKey  = "refreshCookie"
)

// legacyExpiryNodes are pre-managedAuth locations that may still carry tokenExpiresAt.
var legacyExpiryNodes = []string{Sub2APIAuthConfigKey, NewApiV1AuthConfigKey}

// SetNewApiV1RefreshCookie stores the rotating refresh cookie used by new-api forks.
// The value may be pasted as a bare cookie value or as a full cookie / Set-Cookie
// header, in which case the new_api_refresh value is extracted.
func SetNewApiV1RefreshCookie(cfg map[string]interface{}, value string) {
	if cfg == nil {
		return
	}
	value = normalizeRefreshCookieInput(value)
	if value == "" {
		ClearNewApiV1RefreshCookie(cfg)
		return
	}
	node, _ := cfg[NewApiV1AuthConfigKey].(map[string]interface{})
	if node == nil {
		node = map[string]interface{}{}
	}
	node[RefreshCookieKey] = value
	cfg[NewApiV1AuthConfigKey] = node
}

// ClearNewApiV1RefreshCookie removes the refresh cookie, dropping the node when empty.
func ClearNewApiV1RefreshCookie(cfg map[string]interface{}) {
	if cfg == nil {
		return
	}
	node, ok := cfg[NewApiV1AuthConfigKey].(map[string]interface{})
	if !ok || node == nil {
		return
	}
	delete(node, RefreshCookieKey)
	if len(node) == 0 {
		delete(cfg, NewApiV1AuthConfigKey)
	} else {
		cfg[NewApiV1AuthConfigKey] = node
	}
}

func normalizeRefreshCookieInput(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, newApiV1RefreshCookieName+"=") {
		if extracted, ok := CookieValueFromHeader(value, newApiV1RefreshCookieName); ok {
			return strings.TrimSpace(extracted)
		}
	}
	return value
}

// NormalizeEpochMillis accepts a unix timestamp in seconds or milliseconds and
// returns milliseconds. 1e12 ms is 2001-09-09, so smaller values are seconds.
func NormalizeEpochMillis(value int64) int64 {
	if value > 0 && value < 1_000_000_000_000 {
		return value * 1000
	}
	return value
}

// SetManagedTokenExpiresAt writes the canonical expiry (unix millis) into cfg and
// removes legacy copies, so readers can never observe two different values.
func SetManagedTokenExpiresAt(cfg map[string]interface{}, expiresAtMillis int64) {
	if cfg == nil || expiresAtMillis <= 0 {
		return
	}
	node, _ := cfg[ManagedAuthConfigKey].(map[string]interface{})
	if node == nil {
		node = map[string]interface{}{}
	}
	node[TokenExpiresAtKey] = expiresAtMillis
	cfg[ManagedAuthConfigKey] = node
	dropLegacyExpiry(cfg)
}

// ClearManagedTokenExpiresAt removes the canonical expiry and every legacy copy.
func ClearManagedTokenExpiresAt(cfg map[string]interface{}) {
	if cfg == nil {
		return
	}
	if node, ok := cfg[ManagedAuthConfigKey].(map[string]interface{}); ok && node != nil {
		delete(node, TokenExpiresAtKey)
		if len(node) == 0 {
			delete(cfg, ManagedAuthConfigKey)
		} else {
			cfg[ManagedAuthConfigKey] = node
		}
	}
	dropLegacyExpiry(cfg)
}

func dropLegacyExpiry(cfg map[string]interface{}) {
	for _, key := range legacyExpiryNodes {
		node, ok := cfg[key].(map[string]interface{})
		if !ok || node == nil {
			continue
		}
		if _, exists := node[TokenExpiresAtKey]; !exists {
			continue
		}
		delete(node, TokenExpiresAtKey)
		if len(node) == 0 {
			delete(cfg, key)
		} else {
			cfg[key] = node
		}
	}
}
