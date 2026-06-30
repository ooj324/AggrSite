package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"metapi/aggrsite/db"
	"metapi/aggrsite/platform"
	"net/url"
	"strings"
)

// ---- Helpers ----

func nullStr(ns *string) string {
	if ns != nil {
		return *ns
	}
	return ""
}

func resolvePlatformUserID(extraConfig *string) int64 {
	if extraConfig == nil || *extraConfig == "" {
		return 0
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(*extraConfig), &cfg); err != nil {
		return 0
	}
	if id, ok := cfg["platformUserId"].(float64); ok && id > 0 {
		return int64(id)
	}
	return 0
}

// ---- Checkin ----

type CheckinAccountResult struct {
	Success bool   `json:"success"`
	Status  string `json:"status"` // success | failed | skipped
	Message string `json:"message"`
	Reward  string `json:"reward,omitempty"`
}

func isAlreadyCheckedIn(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	patterns := []string{
		"already checked in", "already signed", "already sign in",
		"今日已签到", "今天已签到", "今天已经签到", "今日已经签到",
		"已经签到", "已签到", "重复签到", "签到过",
	}
	for _, p := range patterns {
		if strings.Contains(text, p) {
			return true
		}
	}
	return false
}

func isUnsupportedCheckin(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	patterns := []string{
		"invalid url (post /api/user/checkin)",
		"checkin endpoint not found",
		"check-in is not supported",
		"checkin is not supported",
		"does not support checkin",
		"not support checkin",
	}
	for _, p := range patterns {
		if strings.Contains(text, p) {
			return true
		}
	}
	if strings.Contains(text, "http 404") && strings.Contains(text, "/api/user/checkin") {
		return true
	}
	return false
}

func isAgentRouterPlatform(value string) bool {
	normalized := strings.NewReplacer("-", "", "_", "", " ", "").Replace(strings.ToLower(strings.TrimSpace(value)))
	return normalized == "agentrouter"
}

type ExternalCheckinConfig struct {
	Method     string `json:"method"`
	URL        string `json:"url"`
	AuthHeader string `json:"auth_header"`
	AuthPrefix string `json:"auth_prefix"`
	Body       string `json:"body"`
}

func originFromURL(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func stripAuthBearerPrefix(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) >= 7 && strings.EqualFold(trimmed[:7], "Bearer ") {
		return strings.TrimSpace(trimmed[7:])
	}
	return trimmed
}

func buildCookieAuthHeaderValue(credential, prefix string) string {
	cleanCredential := stripAuthBearerPrefix(credential)
	cleanPrefix := strings.TrimSpace(prefix)

	if cleanPrefix == "" {
		if normalized := platform.NormalizeCookieHeader(cleanCredential); normalized != "" {
			return normalized
		}
		return cleanCredential
	}

	if strings.HasSuffix(cleanPrefix, "=") && !strings.Contains(cleanPrefix, ";") {
		cookieName := strings.TrimSpace(strings.TrimSuffix(cleanPrefix, "="))
		if cookieName != "" {
			if value, ok := platform.CookieValueFromHeader(cleanCredential, cookieName); ok {
				return platform.NormalizeCookieHeader(cookieName + "=" + value)
			}
			if value, ok := platform.CookieCredentialValueFromHeader(cleanCredential); ok {
				return platform.NormalizeCookieHeader(cookieName + "=" + value)
			}
		}
	}

	candidate := cleanPrefix + cleanCredential
	if normalized := platform.NormalizeCookieHeader(candidate); normalized != "" {
		return normalized
	}
	return candidate
}

func doGenericCheckin(config ExternalCheckinConfig, credential string, opt *platform.RequestOption) (*platform.CheckinResult, error) {
	base := platform.BaseAdapter{}
	headers := map[string]string{
		"Content-Type":     "application/json",
		"Accept":           "application/json, text/plain, */*",
		"X-Requested-With": "XMLHttpRequest",
	}
	if origin := originFromURL(config.URL); origin != "" {
		headers["Origin"] = origin
		headers["Referer"] = origin + "/"
	}

	if !strings.EqualFold(strings.TrimSpace(config.AuthHeader), "none") {
		authHeader := strings.TrimSpace(config.AuthHeader)
		authPrefix := config.AuthPrefix
		if authHeader == "" {
			if platform.IsCookieSessionToken(credential) {
				authHeader = "Cookie"
				authPrefix = ""
			} else {
				// No header name specified — fall back to the default Authorization: Bearer
				authHeader = "Authorization"
				authPrefix = "Bearer "
			}
		}
		if strings.EqualFold(authHeader, "Cookie") {
			headers["Cookie"] = buildCookieAuthHeaderValue(credential, authPrefix)
		} else {
			headers[authHeader] = authPrefix + stripAuthBearerPrefix(credential)
		}
	}

	// Build the request body. Many check-in endpoints sit behind gateways/CDNs that
	// expect a body when Content-Type is application/json; sending none makes them
	// hang waiting for a body until the client times out. Default to an empty JSON
	// object unless the site specifies a custom body.
	var body interface{}
	method := strings.ToUpper(strings.TrimSpace(config.Method))
	if method == "POST" || method == "PUT" || method == "PATCH" {
		raw := strings.TrimSpace(config.Body)
		if strings.EqualFold(raw, "none") {
			raw = ""
		} else if raw == "" {
			raw = "{}"
		}
		if raw != "" {
			var parsed interface{}
			if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
				body = parsed
			} else {
				body = json.RawMessage(raw)
			}
		}
	}

	var res map[string]interface{}
	err := base.FetchJSON(config.URL, config.Method, headers, body, &res, opt)
	result := &platform.CheckinResult{}
	if err != nil {
		result.Success = false
		result.Message = err.Error()
		return result, nil
	}
	msg := platform.ExtractMessage(res)
	if msg == "" {
		msg = "check-in override executed"
	}

	// Check for a logical success flag in the response JSON
	isSuccess := true
	if successVal, exists := res["success"]; exists {
		if b, ok := successVal.(bool); ok {
			isSuccess = b
		}
	}

	result.Success = isSuccess
	result.Message = msg
	return result, nil
}

func tryAgentRouterLoginCheckin(row db.AccountWithSite, adapter platform.Adapter, opt *platform.RequestOption) (*platform.CheckinResult, string) {
	if row.ExtraConfig == nil || *row.ExtraConfig == "" {
		return nil, ""
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(*row.ExtraConfig), &cfg); err != nil {
		return nil, ""
	}

	autoRelogin, ok := cfg["autoRelogin"].(map[string]interface{})
	if !ok {
		return nil, ""
	}

	username, _ := autoRelogin["username"].(string)
	passwordCipher, _ := autoRelogin["passwordCipher"].(string)
	if username == "" || passwordCipher == "" {
		return nil, ""
	}

	password := DecryptPassword(passwordCipher)
	if password == "" {
		return &platform.CheckinResult{Success: false, Message: "AgentRouter login checkin failed: stored password cannot be decrypted"}, ""
	}

	loginResult, err := adapter.Login(row.SiteURL, username, password, opt)
	if err != nil {
		return &platform.CheckinResult{Success: false, Message: err.Error()}, ""
	}
	if loginResult == nil || !loginResult.Success || loginResult.AccessToken == "" {
		msg := "AgentRouter login checkin failed"
		if loginResult != nil && loginResult.Message != "" {
			msg = loginResult.Message
		}
		return &platform.CheckinResult{Success: false, Message: msg}, ""
	}

	updates := map[string]interface{}{
		"access_token": loginResult.AccessToken,
	}
	if loginResult.PlatformUserID > 0 {
		cfg["platformUserId"] = loginResult.PlatformUserID
		if cfgBytes, err := json.Marshal(cfg); err == nil {
			updates["extra_config"] = string(cfgBytes)
		}
	}
	_ = db.UpdateAccount(row.ID, updates)

	if strings.Contains(loginResult.Message, "签到") {
		return &platform.CheckinResult{Success: true, Message: loginResult.Message}, loginResult.AccessToken
	}

	platformUserID := resolvePlatformUserID(row.ExtraConfig)
	if loginResult.PlatformUserID > 0 {
		platformUserID = loginResult.PlatformUserID
	}
	result, err := adapter.Checkin(row.SiteURL, loginResult.AccessToken, platformUserID, opt)
	if err != nil {
		return &platform.CheckinResult{Success: false, Message: err.Error()}, loginResult.AccessToken
	}
	return result, loginResult.AccessToken
}

func CheckinAccount(accountID int64) (*CheckinAccountResult, error) {
	row, err := db.GetAccountWithSite(accountID)
	if err != nil {
		return nil, fmt.Errorf("account not found: %w", err)
	}

	// Skip disabled sites
	if row.SiteStatus == "disabled" {
		_ = db.UpdateAccount(accountID, map[string]interface{}{
			"extra_config": mergeRuntimeHealth(row.ExtraConfig, "disabled", "站点已禁用", "checkin"),
		})
		_ = db.InsertCheckinLog(accountID, "skipped", "site disabled", "")
		slog.Info("Checkin skipped: site disabled", "account_id", accountID)
		return &CheckinAccountResult{Success: true, Status: "skipped", Message: "site disabled"}, nil
	}

	adapter := platform.GetAdapter(row.SitePlatform)
	if adapter == nil {
		msg := "unsupported platform: " + row.SitePlatform
		_ = db.UpdateAccount(accountID, map[string]interface{}{
			"extra_config": mergeRuntimeHealth(row.ExtraConfig, "unhealthy", msg, "checkin"),
		})
		_ = db.InsertCheckinLog(accountID, "skipped", msg, "")
		return &CheckinAccountResult{Success: false, Status: "skipped", Message: msg}, nil
	}

	checkinURL := row.SiteURL
	hasOverrideURL := false
	var overrideConfig ExternalCheckinConfig
	useGeneric := false

	if row.SiteExternalCheckinURL != nil && *row.SiteExternalCheckinURL != "" {
		hasOverrideURL = true
		useGeneric = true
		checkinURL = strings.TrimSpace(*row.SiteExternalCheckinURL)

		overrideConfig.URL = checkinURL

		if row.SiteExternalCheckinBody != nil {
			overrideConfig.Body = *row.SiteExternalCheckinBody
		}

		if row.SiteExternalCheckinMethod != nil && *row.SiteExternalCheckinMethod != "" {
			overrideConfig.Method = *row.SiteExternalCheckinMethod

			// Only respect AuthHeader/AuthPrefix if Method is also provided (Advanced Mode)
			if row.SiteExternalCheckinAuthHeader != nil {
				overrideConfig.AuthHeader = *row.SiteExternalCheckinAuthHeader
			} else {
				overrideConfig.AuthHeader = "Authorization"
			}

			if row.SiteExternalCheckinAuthPrefix != nil {
				overrideConfig.AuthPrefix = *row.SiteExternalCheckinAuthPrefix
			} else {
				overrideConfig.AuthPrefix = "Bearer "
			}
		} else {
			// Simple Mode: default auth header is selected from the credential type.
			overrideConfig.Method = "POST"
			overrideConfig.AuthHeader = ""
			overrideConfig.AuthPrefix = ""
		}
	}

	opt := &platform.RequestOption{
		ProxyURL:       row.SiteProxyURL,
		UseSystemProxy: row.SiteUseSystemProxy,
		CustomHeaders:  row.SiteCustomHeaders,
	}

	checkinCredential := row.AccessToken
	if row.ExtraConfig != nil && *row.ExtraConfig != "" {
		var cfg map[string]interface{}
		if err := json.Unmarshal([]byte(*row.ExtraConfig), &cfg); err == nil {
			if proxyUrl, ok := cfg["proxyUrl"].(string); ok && proxyUrl != "" {
				opt.ProxyURL = &proxyUrl
			}
			if useSystemProxy, ok := cfg["useSystemProxy"].(bool); ok {
				opt.UseSystemProxy = &useSystemProxy
			}
			if cred, ok := cfg["checkin_credential"].(string); ok && cred != "" {
				checkinCredential = cred
			}
		}
	}

	platformUserID := resolvePlatformUserID(row.ExtraConfig)

	executeCheckin := func(token string, overrideCred string) (*platform.CheckinResult, error) {
		if useGeneric {
			return doGenericCheckin(overrideConfig, overrideCred, opt)
		}

		if isAgentRouterPlatform(row.SitePlatform) {
			if result, refreshedToken := tryAgentRouterLoginCheckin(*row, adapter, opt); result != nil {
				if refreshedToken != "" {
					row.AccessToken = refreshedToken
				}
				return result, nil
			}
		}

		// Try adapter first (reuses main site auth method)
		res, err := adapter.Checkin(checkinURL, token, platformUserID, opt)

		// If adapter returns unsupported, AND we have an override URL, fallback to generic checkin
		if err == nil && res != nil && hasOverrideURL && isUnsupportedCheckin(res.Message) {
			slog.Info("Adapter checkin unsupported, falling back to generic checkin", "url", overrideConfig.URL)
			return doGenericCheckin(overrideConfig, overrideCred, opt)
		}

		return res, err
	}

	previousBalance := valueOrZero(row.Balance)
	result, err := executeCheckin(row.AccessToken, checkinCredential)
	if err != nil {
		result = &platform.CheckinResult{Success: false, Message: err.Error()}
	}

	alreadyCheckedIn := isAlreadyCheckedIn(result.Message)
	unsupported := isUnsupportedCheckin(result.Message)

	failureReason := AnalyzeCheckinFailure(result.Message)
	turnstileRequired := failureReason.Code == "TURNSTILE_REQUIRED"

	effectiveSuccess := result.Success || alreadyCheckedIn || unsupported || turnstileRequired

	var status string
	switch {
	case unsupported || turnstileRequired:
		status = "skipped"
	case effectiveSuccess:
		status = "success"
	default:
		// Attempt auto-relogin ONLY if token is expired
		if failureReason.Code == "TOKEN_EXPIRED" {
			if refreshedAccessToken := tryAutoRelogin(*row, adapter, opt); refreshedAccessToken != "" {
				row.AccessToken = refreshedAccessToken
				// Retry checkin
				result, err = executeCheckin(row.AccessToken, checkinCredential)
				if err != nil {
					result = &platform.CheckinResult{Success: false, Message: err.Error()}
				}

				alreadyCheckedIn = isAlreadyCheckedIn(result.Message)
				unsupported = isUnsupportedCheckin(result.Message)
				newFailure := AnalyzeCheckinFailure(result.Message)
				turnstileRequired = newFailure.Code == "TURNSTILE_REQUIRED"

				effectiveSuccess = result.Success || alreadyCheckedIn || unsupported || turnstileRequired
				if unsupported || turnstileRequired {
					status = "skipped"
				} else if effectiveSuccess {
					status = "success"
				} else {
					status = "failed"
				}
			} else {
				status = "failed"
			}
		} else {
			status = "failed"
		}
	}

	directSuccess := result.Success && !alreadyCheckedIn && !unsupported && !turnstileRequired
	var refreshedBalance *BalanceResult
	if effectiveSuccess && !unsupported && !turnstileRequired {
		refreshedBalance, _ = RefreshBalance(accountID)
	}

	// Reward inference if success but no explicit reward. Use the persisted balance
	// captured before check-in and refresh the main site balance afterward; do not
	// call balance APIs against an external check-in URL.
	if directSuccess && result.Reward == "" && refreshedBalance != nil && refreshedBalance.Balance != nil {
		delta := refreshedBalance.Balance.Balance - previousBalance
		if delta > 0 {
			result.Reward = fmt.Sprintf("%.6g", delta)
		}
	}

	// Write checkin log
	_ = db.InsertCheckinLog(accountID, status, result.Message, result.Reward)

	// Write event
	eventLevel := "info"
	eventTitle := "checkin " + status
	if status == "failed" {
		eventLevel = "error"
	}
	_ = db.InsertEvent("checkin", eventTitle,
		fmt.Sprintf("%s @ %s: %s", nullStr(row.Username), row.SiteName, result.Message),
		eventLevel, &accountID, "account")

	updates := map[string]interface{}{}
	if directSuccess {
		updates["last_checkin_at"] = db.TimeNow()
	}
	if effectiveSuccess {
		healthState := "healthy"
		healthReason := result.Message
		if unsupported || turnstileRequired {
			healthState = "degraded"
		}
		if healthReason == "" {
			healthReason = "签到成功"
		}
		updates["extra_config"] = mergeRuntimeHealth(row.ExtraConfig, healthState, healthReason, "checkin")
		if row.Status != nil && *row.Status == "expired" && status == "success" {
			updates["status"] = "active"
		}
	} else {
		updates["extra_config"] = mergeRuntimeHealth(row.ExtraConfig, "unhealthy", result.Message, "checkin")
		if AnalyzeCheckinFailure(result.Message).Code == "TOKEN_EXPIRED" {
			updates["status"] = "expired"
		}
	}
	if len(updates) > 0 {
		_ = db.UpdateAccount(accountID, updates)
	}

	slog.Info("Checkin completed", "account_id", accountID, "status", status, "message", result.Message)
	return &CheckinAccountResult{
		Success: effectiveSuccess,
		Status:  status,
		Message: result.Message,
		Reward:  result.Reward,
	}, nil
}

func tryAutoRelogin(row db.AccountWithSite, adapter platform.Adapter, opt *platform.RequestOption) string {
	if row.ExtraConfig == nil || *row.ExtraConfig == "" {
		return ""
	}

	// 1. Try RefreshAuth first (e.g. sub2api with refreshToken)
	if isSub2APIPlatform(row.SitePlatform) {
		refreshedAccessToken, _, didRefresh, err := forceRefreshSub2APIManagedSession(row, opt)
		if err == nil && refreshedAccessToken != "" && (didRefresh || refreshedAccessToken != row.AccessToken) {
			return refreshedAccessToken
		}
		if err != nil {
			slog.Warn("Sub2API managed RefreshAuth failed", "account_id", row.ID, "err", err)
		}
		return ""
	}

	refreshRes, err := adapter.RefreshAuth(row.SiteURL, row.AccessToken, *row.ExtraConfig, opt)
	if err == nil && refreshRes != nil && refreshRes.Success && refreshRes.AccessToken != "" {
		slog.Info("Auto RefreshAuth successful, updating access token and config", "account_id", row.ID)
		_ = db.UpdateAccount(row.ID, map[string]interface{}{
			"access_token": refreshRes.AccessToken,
			"extra_config": refreshRes.ExtraConfig,
		})
		return refreshRes.AccessToken
	}

	// 2. Fallback to classic username/password Login
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(*row.ExtraConfig), &cfg); err != nil {
		return ""
	}

	autoRelogin, ok := cfg["autoRelogin"].(map[string]interface{})
	if !ok {
		return ""
	}

	username, _ := autoRelogin["username"].(string)
	passwordCipher, _ := autoRelogin["passwordCipher"].(string)

	if username == "" || passwordCipher == "" {
		return ""
	}

	password := DecryptPassword(passwordCipher)
	if password == "" {
		return ""
	}

	slog.Info("Attempting auto-relogin via username/password", "account_id", row.ID)
	loginResult, err := adapter.Login(row.SiteURL, username, password, opt)
	if err != nil || loginResult == nil || !loginResult.Success || loginResult.AccessToken == "" {
		slog.Warn("Auto-relogin failed", "account_id", row.ID, "err", err, "message", loginResult.Message)
		return ""
	}

	slog.Info("Auto-relogin via Login successful, updating access token", "account_id", row.ID)
	updates := map[string]interface{}{
		"access_token": loginResult.AccessToken,
	}

	if loginResult.PlatformUserID > 0 {
		if pid, ok := cfg["platformUserId"].(float64); !ok || pid <= 0 {
			cfg["platformUserId"] = loginResult.PlatformUserID
			if cfgBytes, err := json.Marshal(cfg); err == nil {
				updates["extra_config"] = string(cfgBytes)
			}
		}
	}

	_ = db.UpdateAccount(row.ID, updates)

	return loginResult.AccessToken
}

type CheckinAllResult struct {
	AccountID int64                 `json:"account_id"`
	Username  string                `json:"username"`
	Site      string                `json:"site"`
	Result    *CheckinAccountResult `json:"result"`
}

func CheckinAll() ([]CheckinAllResult, error) {
	rows, err := db.ListCheckinableAccounts()
	if err != nil {
		return nil, err
	}

	var results []CheckinAllResult
	for _, row := range rows {
		r, _ := CheckinAccount(row.ID)
		if r == nil {
			r = &CheckinAccountResult{Success: false, Status: "failed", Message: "internal error"}
		}
		results = append(results, CheckinAllResult{
			AccountID: row.ID,
			Username:  nullStr(row.Username),
			Site:      row.SiteName,
			Result:    r,
		})
	}

	return results, nil
}
