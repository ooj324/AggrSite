package service

import (
	"encoding/json"
	"metapi/aggrsite/db"
	"strings"
	"time"
)

func mergeAccountExtraConfig(extraConfig *string, values map[string]interface{}) string {
	cfg := map[string]interface{}{}
	if extraConfig != nil && *extraConfig != "" {
		_ = json.Unmarshal([]byte(*extraConfig), &cfg)
	}
	for k, v := range values {
		if v == nil {
			delete(cfg, k)
		} else {
			cfg[k] = v
		}
	}
	bs, _ := json.Marshal(cfg)
	return string(bs)
}

func buildRuntimeHealth(state, reason, source string) map[string]interface{} {
	return map[string]interface{}{
		"state":     state,
		"reason":    reason,
		"source":    source,
		"updatedAt": time.Now().UTC().Format(time.RFC3339),
	}
}

func mergeRuntimeHealth(extraConfig *string, state, reason, source string) string {
	return mergeAccountExtraConfig(extraConfig, map[string]interface{}{
		"runtimeHealth": buildRuntimeHealth(state, reason, source),
	})
}

// freshRuntimeHealth re-reads extra_config from the database before merging the
// runtimeHealth update. This prevents a stale in-memory snapshot from overwriting
// concurrent updates (e.g. a background scheduler refresh that rotated the
// refresh cookie while a long-running checkin or balance operation was in flight).
// If the fresh read fails, the provided fallback snapshot is used instead.
func freshRuntimeHealth(accountID int64, fallback *string, state, reason, source string) string {
	fresh := fallback
	if row, err := db.GetAccount(accountID); err == nil && row != nil && row.ExtraConfig != nil {
		fresh = row.ExtraConfig
	}
	return mergeRuntimeHealth(fresh, state, reason, source)
}

func isApiKeyAccount(accessToken string, apiToken *string, extraConfig *string) bool {
	if extraConfig != nil && *extraConfig != "" {
		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(*extraConfig), &cfg); err == nil {
			if mode, ok := cfg["credentialMode"].(string); ok && mode != "" && mode != "auto" {
				return mode == "apikey"
			}
		}
	}
	return strings.TrimSpace(accessToken) == "" && apiToken != nil && strings.TrimSpace(*apiToken) != ""
}
