package db

import (
	"encoding/json"
	"strings"
)

// Archive markers stored inside accounts.extra_config when a site is disabled.
// archivedBySite marks the row as cascade-archived (distinct from a manual
// disable); archivedPrevStatus snapshots the status at archive time so the
// restore can recover it losslessly (active stays active, expired stays
// expired).
const (
	ExtraKeyArchivedBySite     = "archivedBySite"
	ExtraKeyArchivedPrevStatus = "archivedPrevStatus"
)

// ---- Normalized status + display-state computation (single source of truth) ----
//
// The frontend used to re-derive a runtime state from status + extra_config on
// its own (and got the mapping subtly wrong, e.g. backend writes "unhealthy"
// while the UI filter expected "abnormal"). Both the state and its label are
// now computed here and shipped with the account payload, so the UI only
// renders what it is given.

const (
	DisplayStateHealthy  = "healthy"
	DisplayStateDegraded = "degraded"
	DisplayStateAbnormal = "abnormal"
	DisplayStateDisabled = "disabled"
	DisplayStateArchived = "archived"
)

// AccountDisplay is the canonical, UI-ready representation of an account's
// current state. State is the machine-readable filter value; Label/Reason/
// Source are ready for display as-is.
type AccountDisplay struct {
	State  string `json:"state"`
	Label  string `json:"label"`
	Reason string `json:"reason"`
	Source string `json:"source,omitempty"`
}

// NormalizeAccountStatus trims/lowercases an account status and treats NULL or
// empty as active, matching the COALESCE fallbacks used in scheduler queries.
func NormalizeAccountStatus(status *string) string {
	if status == nil {
		return "active"
	}
	s := strings.ToLower(strings.TrimSpace(*status))
	if s == "" {
		return "active"
	}
	return s
}

// IsAccountArchived reports whether the account was cascade-archived by a site
// disable (as opposed to disabled manually).
func IsAccountArchived(extraConfig *string) bool {
	if extraConfig == nil {
		return false
	}
	v, _ := ParseExtraConfigFlag(*extraConfig, ExtraKeyArchivedBySite).(bool)
	return v
}

// ParseExtraConfigFlag extracts one key from an extra_config JSON string.
func ParseExtraConfigFlag(extraConfig, key string) interface{} {
	cfg := map[string]interface{}{}
	if strings.TrimSpace(extraConfig) != "" {
		_ = json.Unmarshal([]byte(extraConfig), &cfg)
	}
	return cfg[key]
}

func runtimeHealthOf(extraConfig *string) (state, reason, source string) {
	if extraConfig != nil && strings.TrimSpace(*extraConfig) != "" {
		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(*extraConfig), &cfg); err == nil {
			if rh, ok := cfg["runtimeHealth"].(map[string]interface{}); ok {
				state, _ = rh["state"].(string)
				reason, _ = rh["reason"].(string)
				source, _ = rh["source"].(string)
			}
		}
	}
	return
}

// NormalizeRuntimeReason strips the technical prefixes produced by upstream
// errors so the reason reads cleanly in the UI. Centralizes the rule that used
// to live in the frontend.
func NormalizeRuntimeReason(reason string) string {
	text := strings.TrimSpace(reason)
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	for _, prefix := range []string{"failed to fetch balance:", "failed:", "error:"} {
		if strings.HasPrefix(lower, prefix) {
			text = strings.TrimSpace(text[len(prefix):])
			break
		}
	}
	return text
}

// runtimeSourceLabel maps the runtimeHealth source machine key to its label.
func runtimeSourceLabel(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "checkin":
		return "签到"
	case "balance":
		return "余额"
	case "login":
		return "登录"
	case "verify":
		return "验证"
	case "system":
		return "系统"
	default:
		return ""
	}
}

// ComputeAccountDisplay resolves the single display state for an account.
// Precedence (first match wins):
//
//	archived  — extra_config.archivedBySite (site cascade)
//	disabled  — manual disable (status = disabled, no archive marker)
//	abnormal  — status = expired (label 令牌失效) or runtime unhealthy
//	degraded  — runtimeHealth.state = degraded
//	healthy   — everything else, including runtime healthy
//
// Runtime health never re-promotes a disabled account to healthy; status wins.
func ComputeAccountDisplay(status *string, extraConfig *string) AccountDisplay {
	normalized := NormalizeAccountStatus(status)
	rhState, rhReason, rhSource := runtimeHealthOf(extraConfig)
	reason := NormalizeRuntimeReason(rhReason)
	source := runtimeSourceLabel(rhSource)

	if IsAccountArchived(extraConfig) {
		if reason == "" {
			reason = "站点已禁用，账号已自动归档"
		}
		return AccountDisplay{State: DisplayStateArchived, Label: "已归档", Reason: reason, Source: source}
	}

	switch normalized {
	case "disabled":
		if reason == "" {
			reason = "账号已禁用"
		}
		return AccountDisplay{State: DisplayStateDisabled, Label: "禁用", Reason: reason, Source: source}
	case "expired":
		if reason == "" {
			reason = "令牌失效"
		}
		return AccountDisplay{State: DisplayStateAbnormal, Label: "令牌失效", Reason: reason, Source: source}
	}

	// status == active (or unknown): runtime health decides.
	switch strings.ToLower(strings.TrimSpace(rhState)) {
	case "degraded":
		if reason == "" {
			reason = "需关注"
		}
		return AccountDisplay{State: DisplayStateDegraded, Label: "需关注", Reason: reason, Source: source}
	case "unhealthy", "abnormal":
		if reason == "" {
			reason = "异常"
		}
		return AccountDisplay{State: DisplayStateAbnormal, Label: "异常", Reason: reason, Source: source}
	case "healthy", "":
		if reason == "" {
			reason = "正常"
		}
		return AccountDisplay{State: DisplayStateHealthy, Label: "正常", Reason: reason, Source: source}
	default:
		// Unknown runtime state on an active account: treat as abnormal so it is
		// visible instead of silently looking healthy.
		if reason == "" {
			reason = "异常"
		}
		return AccountDisplay{State: DisplayStateAbnormal, Label: "异常", Reason: reason, Source: source}
	}
}

// ---- Archive / restore cascade ----

func mergeExtraConfigKeyed(raw *string, set map[string]interface{}, unset ...string) string {
	cfg := map[string]interface{}{}
	if raw != nil && strings.TrimSpace(*raw) != "" {
		_ = json.Unmarshal([]byte(*raw), &cfg)
	}
	for _, k := range unset {
		delete(cfg, k)
	}
	for k, v := range set {
		cfg[k] = v
	}
	bs, _ := json.Marshal(cfg)
	return string(bs)
}

func archivedPrevStatus(raw *string) string {
	v, _ := ParseExtraConfigFlag(orEmpty(raw), ExtraKeyArchivedPrevStatus).(string)
	v = strings.ToLower(strings.TrimSpace(v))
	// Only scheduler-meaningful statuses are restorable; anything else defers to active.
	if v == "active" || v == "expired" {
		return v
	}
	return "active"
}

func orEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ArchiveAccountsBySite cascade-archives every non-disabled account of a site:
// status becomes "disabled" and the previous status is snapshotted into
// extra_config so RestoreAccountsBySite can recover it exactly. Accounts the
// user disabled manually (already disabled, no marker) are untouched, as are
// accounts already archived by an earlier disable. Returns the number of
// accounts archived on this call.
func ArchiveAccountsBySite(siteID int64) (int64, error) {
	var accounts []Account
	if err := Select(&accounts, `SELECT `+accountColumns+` FROM accounts WHERE site_id = ? ORDER BY id ASC`, siteID); err != nil {
		return 0, err
	}
	var count int64
	for _, acc := range accounts {
		if IsAccountArchived(acc.ExtraConfig) {
			continue
		}
		status := NormalizeAccountStatus(acc.Status)
		if status == "disabled" || status == "" {
			continue // manual disable (or nothing enabled) — never snapshot those
		}
		newCfg := mergeExtraConfigKeyed(acc.ExtraConfig, map[string]interface{}{
			ExtraKeyArchivedBySite:     true,
			ExtraKeyArchivedPrevStatus: status,
		})
		if err := UpdateAccount(acc.ID, map[string]interface{}{
			"status":       "disabled",
			"extra_config": newCfg,
		}); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// RestoreAccountsBySite reverses ArchiveAccountsBySite: only marked rows are
// restored, each back to its snapshotted pre-archive status. Manual disables
// (no marker) are preserved, so user intent is never clobbered. Returns the
// number of accounts restored.
func RestoreAccountsBySite(siteID int64) (int64, error) {
	var accounts []Account
	if err := Select(&accounts, `SELECT `+accountColumns+` FROM accounts WHERE site_id = ? ORDER BY id ASC`, siteID); err != nil {
		return 0, err
	}
	var count int64
	for _, acc := range accounts {
		if !IsAccountArchived(acc.ExtraConfig) {
			continue
		}
		prevStatus := archivedPrevStatus(acc.ExtraConfig)
		newCfg := mergeExtraConfigKeyed(acc.ExtraConfig, nil, ExtraKeyArchivedBySite, ExtraKeyArchivedPrevStatus)
		if err := UpdateAccount(acc.ID, map[string]interface{}{
			"status":       prevStatus,
			"extra_config": newCfg,
		}); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// SetAccountStatusUserDriven applies an explicit user-driven status change
// (single edit or batch enable/disable). It clears any cascade-archive marker,
// because an explicit user choice supersedes the site snapshot: a manually
// re-enabled archived account must not be flipped back to its snapshot status
// when the site is enabled later.
func SetAccountStatusUserDriven(id int64, status string) error {
	acc, err := GetAccount(id)
	if err != nil {
		return err
	}
	fields := map[string]interface{}{"status": status}
	if IsAccountArchived(acc.ExtraConfig) {
		fields["extra_config"] = mergeExtraConfigKeyed(acc.ExtraConfig, nil, ExtraKeyArchivedBySite, ExtraKeyArchivedPrevStatus)
	}
	return UpdateAccount(id, fields)
}

// CountArchivedAccountsBySite reports how many accounts of the site currently
// carry the cascade-archive marker (used for events / messaging).
func CountArchivedAccountsBySite(siteID int64) (int64, error) {
	var accounts []Account
	if err := Select(&accounts, `SELECT `+accountColumns+` FROM accounts WHERE site_id = ?`, siteID); err != nil {
		return 0, err
	}
	var count int64
	for _, acc := range accounts {
		if IsAccountArchived(acc.ExtraConfig) {
			count++
		}
	}
	return count, nil
}
