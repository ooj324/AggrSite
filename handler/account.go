package handler

import (
	"encoding/json"
	"metapi/aggrsite/db"
	"metapi/aggrsite/platform"
	"metapi/aggrsite/service"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

func isMaskedTokenValue(token string) bool {
	return strings.Contains(token, "*") || strings.Contains(token, "•")
}

func getRequestOption(site *db.Site) *platform.RequestOption {
	return &platform.RequestOption{
		ProxyURL:       site.ProxyURL,
		UseSystemProxy: site.UseSystemProxy,
		CustomHeaders:  site.CustomHeaders,
	}
}

func VerifyToken(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SiteID         int64   `json:"siteId"`
		AccessToken    string  `json:"accessToken"`
		PlatformUserID *int64  `json:"platformUserId"`
		CredentialMode string  `json:"credentialMode"`
		ProxyURL       *string `json:"proxyUrl"`
		UseSystemProxy *bool   `json:"useSystemProxy"`
	}
	if err := parseBody(r, &input); err != nil {
		fail(w, http.StatusBadRequest, "invalid body")
		return
	}

	site, err := db.GetSite(input.SiteID)
	if err != nil {
		fail(w, http.StatusNotFound, "site not found")
		return
	}

	ad := platform.GetAdapter(site.Platform)
	if ad == nil {
		fail(w, http.StatusBadRequest, "unknown platform")
		return
	}

	opt := getRequestOption(site)
	if input.ProxyURL != nil {
		opt.ProxyURL = input.ProxyURL
	}
	if input.UseSystemProxy != nil {
		opt.UseSystemProxy = input.UseSystemProxy
	}

	// Mode handling
	credentialMode := input.CredentialMode
	if credentialMode == "" {
		credentialMode = "session"
	}

	var platformUserID int64
	if input.PlatformUserID != nil {
		platformUserID = *input.PlatformUserID
	}

	if credentialMode == "apikey" {
		// API Key mode: just test if we can fetch models
		models, err := ad.GetModels(site.URL, input.AccessToken, platformUserID, opt)
		if err != nil {
			errStr := err.Error()
			shieldBlocked := strings.Contains(errStr, "acw_sc__v2") || strings.Contains(errStr, "var arg1") || strings.Contains(errStr, "challenge") || strings.Contains(errStr, "cloudflare") || strings.Contains(errStr, "invalid character")
			if shieldBlocked {
				ok(w, map[string]interface{}{"success": false, "shieldBlocked": true, "message": "被反爬或防火墙拦截"})
				return
			}
			fail(w, http.StatusInternalServerError, errStr)
			return
		}
		ok(w, map[string]interface{}{
			"success":    true,
			"tokenType":  "apikey",
			"modelCount": len(models),
			"models":     models,
		})
		return
	}

	res, err := ad.VerifyToken(site.URL, input.AccessToken, platformUserID, opt)
	if err != nil {
		errStr := err.Error()
		// Detect needsUserId or shield
		needsUserId := strings.Contains(errStr, "New-API-User") || strings.Contains(errStr, "user id required")
		shieldBlocked := strings.Contains(errStr, "acw_sc__v2") || strings.Contains(errStr, "var arg1") || strings.Contains(errStr, "challenge") || strings.Contains(errStr, "cloudflare") || strings.Contains(errStr, "invalid character")
		if needsUserId || shieldBlocked {
			ok(w, map[string]interface{}{
				"success":       false,
				"needsUserId":   needsUserId,
				"shieldBlocked": shieldBlocked,
				"message":       errStr,
			})
			return
		}
		fail(w, http.StatusInternalServerError, errStr)
		return
	}

	ok(w, map[string]interface{}{
		"success":   true,
		"tokenType": res.TokenType,
		"userInfo":  res.UserInfo,
		"balance":   res.Balance,
		"apiToken":  res.ApiToken,
	})
}

func RebindSession(w http.ResponseWriter, r *http.Request) {
	id, valid := parseID(r)
	if !valid {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}

	var input struct {
		AccessToken    string  `json:"accessToken"`
		PlatformUserID *int64  `json:"platformUserId"`
		RefreshToken   *string `json:"refreshToken"`
		TokenExpiresAt *int64  `json:"tokenExpiresAt"`
	}
	if err := parseBody(r, &input); err != nil {
		fail(w, http.StatusBadRequest, "invalid body")
		return
	}

	if input.AccessToken == "" {
		fail(w, http.StatusBadRequest, "accessToken is required")
		return
	}

	account, err := db.GetAccount(id)
	if err != nil {
		fail(w, http.StatusNotFound, "account not found")
		return
	}
	site, err := db.GetSite(account.SiteID)
	if err != nil {
		fail(w, http.StatusNotFound, "site not found")
		return
	}

	updates := map[string]interface{}{
		"access_token": input.AccessToken,
	}

	var cfg map[string]interface{}
	if account.ExtraConfig != nil && *account.ExtraConfig != "" {
		json.Unmarshal([]byte(*account.ExtraConfig), &cfg)
	}
	if cfg == nil {
		cfg = make(map[string]interface{})
	}

	platformUserID := int64(0)
	if input.PlatformUserID != nil {
		platformUserID = *input.PlatformUserID
		cfg["platformUserId"] = platformUserID
	} else if existingID, ok := cfg["platformUserId"].(float64); ok {
		platformUserID = int64(existingID)
	}

	if input.RefreshToken != nil {
		sub2apiAuth, _ := cfg["sub2apiAuth"].(map[string]interface{})
		if sub2apiAuth == nil {
			sub2apiAuth = make(map[string]interface{})
		}
		if *input.RefreshToken != "" {
			sub2apiAuth["refreshToken"] = *input.RefreshToken
			if input.TokenExpiresAt != nil {
				sub2apiAuth["tokenExpiresAt"] = *input.TokenExpiresAt
			}
			cfg["sub2apiAuth"] = sub2apiAuth
		} else {
			delete(cfg, "sub2apiAuth")
		}
	}

	cfgBytes, _ := json.Marshal(cfg)
	updates["extra_config"] = string(cfgBytes)

	// Verify the new token
	ad := platform.GetAdapter(site.Platform)
	if ad != nil {
		opt := getRequestOption(site)
		res, err := ad.VerifyToken(site.URL, input.AccessToken, platformUserID, opt)
		if err == nil && res != nil {
			if res.UserInfo != nil && res.UserInfo.Username != "" {
				updates["username"] = res.UserInfo.Username
			}
			if res.ApiToken != "" {
				updates["api_token"] = res.ApiToken
			}
		}
	}

	if err := db.UpdateAccount(id, updates); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	ok(w, map[string]interface{}{"success": true})
}

func ListAccounts(w http.ResponseWriter, r *http.Request) {
	siteID := queryInt64Ptr(r, "siteId")
	accounts, err := db.ListAccountsWithSites(siteID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, accounts)
}

func GetAccount(w http.ResponseWriter, r *http.Request) {
	id, valid := parseID(r)
	if !valid {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}

	account, err := db.GetAccount(id)
	if err != nil {
		fail(w, http.StatusNotFound, "account not found")
		return
	}
	ok(w, account)
}

func CreateAccount(w http.ResponseWriter, r *http.Request) {
	var input struct {
		db.CreateAccountInput
		AccessTokens      []string `json:"accessTokens"`
		SkipModelFetch    *bool    `json:"skipModelFetch"`
		ProxyURL          *string  `json:"proxyUrl"`
		UseSystemProxy    *bool    `json:"useSystemProxy"`
		CheckinCredential *string  `json:"checkin_credential"`
		RefreshToken      *string  `json:"refreshToken"`
		TokenExpiresAt    *int64   `json:"tokenExpiresAt"`
	}
	if err := parseBody(r, &input); err != nil {
		fail(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	if input.SiteID == 0 {
		fail(w, http.StatusBadRequest, "site_id is required")
		return
	}

	if input.AccessToken == "" && input.ApiToken == "" && len(input.AccessTokens) == 0 {
		fail(w, http.StatusBadRequest, "access_token, api_token or accessTokens is required")
		return
	}

	site, err := db.GetSite(input.SiteID)
	if err != nil {
		fail(w, http.StatusNotFound, "site not found")
		return
	}
	ad := platform.GetAdapter(site.Platform)
	if ad == nil {
		fail(w, http.StatusBadRequest, "unknown platform")
		return
	}

	opt := getRequestOption(site)
	if input.ProxyURL != nil {
		opt.ProxyURL = input.ProxyURL
	}
	if input.UseSystemProxy != nil {
		opt.UseSystemProxy = input.UseSystemProxy
	}

	// Batch Processing
	if len(input.AccessTokens) > 0 {
		var createdCount int
		var failedCount int
		var items []map[string]interface{}

		for _, token := range input.AccessTokens {
			if strings.TrimSpace(token) == "" {
				continue
			}
			cloneInput := input.CreateAccountInput
			if input.CredentialMode == "apikey" {
				cloneInput.AccessToken = ""
				cloneInput.ApiToken = token
			} else {
				cloneInput.AccessToken = token
			}

			id, err := db.CreateAccount(cloneInput)
			if err != nil {
				failedCount++
				items = append(items, map[string]interface{}{"status": "failed", "token": token, "message": err.Error()})
				continue
			}

			// Save extra config
			cfg := make(map[string]interface{})
			if input.CredentialMode != "" {
				cfg["credentialMode"] = input.CredentialMode
			}
			if input.PlatformUserID != nil {
				cfg["platformUserId"] = *input.PlatformUserID
			}
			if input.ProxyURL != nil && *input.ProxyURL != "" {
				cfg["proxyUrl"] = *input.ProxyURL
			}
			if input.UseSystemProxy != nil {
				cfg["useSystemProxy"] = *input.UseSystemProxy
			}
			if input.CheckinCredential != nil && *input.CheckinCredential != "" {
				cfg["checkin_credential"] = *input.CheckinCredential
			}
			if input.RefreshToken != nil && *input.RefreshToken != "" {
				sub2apiAuth := make(map[string]interface{})
				sub2apiAuth["refreshToken"] = *input.RefreshToken
				if input.TokenExpiresAt != nil {
					sub2apiAuth["tokenExpiresAt"] = *input.TokenExpiresAt
				}
				cfg["sub2apiAuth"] = sub2apiAuth
			}
			bs, _ := json.Marshal(cfg)
			db.UpdateAccount(id, map[string]interface{}{"extra_config": string(bs)})

			createdCount++
			items = append(items, map[string]interface{}{"status": "success", "id": id, "token": token})
		}

		ok(w, map[string]interface{}{
			"batch":        true,
			"createdCount": createdCount,
			"failedCount":  failedCount,
			"items":        items,
		})
		return
	}

	// Single Processing
	apiTokenFound := false
	usernameDetected := false

	// If missing ApiToken and it's a session token, try to fetch it
	if input.ApiToken == "" && input.AccessToken != "" && input.CredentialMode != "apikey" {
		userID := int64(0)
		if input.PlatformUserID != nil {
			userID = *input.PlatformUserID
		}
		if token, err := ad.GetApiToken(site.URL, input.AccessToken, userID, opt); err == nil {
			input.ApiToken = token
			apiTokenFound = true
		}
	}

	if input.Username != "" {
		usernameDetected = true
	}

	if input.CredentialMode == "apikey" {
		if input.ApiToken == "" {
			input.ApiToken = input.AccessToken
		}
		input.AccessToken = ""
	}

	id, err := db.CreateAccount(input.CreateAccountInput)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Update ExtraConfig with Proxy overrides if present
	cfg := make(map[string]interface{})
	if input.CredentialMode != "" {
		cfg["credentialMode"] = input.CredentialMode
	}
	if input.PlatformUserID != nil {
		cfg["platformUserId"] = *input.PlatformUserID
	}
	if input.ProxyURL != nil && *input.ProxyURL != "" {
		cfg["proxyUrl"] = *input.ProxyURL
	}
	if input.UseSystemProxy != nil {
		cfg["useSystemProxy"] = *input.UseSystemProxy
	}
	if input.CheckinCredential != nil && *input.CheckinCredential != "" {
		cfg["checkin_credential"] = *input.CheckinCredential
	}
	if input.RefreshToken != nil && *input.RefreshToken != "" {
		sub2apiAuth := make(map[string]interface{})
		sub2apiAuth["refreshToken"] = *input.RefreshToken
		if input.TokenExpiresAt != nil {
			sub2apiAuth["tokenExpiresAt"] = *input.TokenExpiresAt
		}
		cfg["sub2apiAuth"] = sub2apiAuth
	}
	bs, _ := json.Marshal(cfg)
	db.UpdateAccount(id, map[string]interface{}{"extra_config": string(bs)})

	// Trigger async sync logic
	go func() {
		userID := int64(0)
		if input.PlatformUserID != nil {
			userID = *input.PlatformUserID
		}
		activeAccessToken := input.AccessToken

		// sync balance
		if input.CredentialMode != "apikey" && activeAccessToken != "" {
			service.RefreshBalance(id)
			if account, err := db.GetAccount(id); err == nil {
				activeAccessToken = account.AccessToken
			}
		}

		// sync tokens
		if input.CredentialMode != "apikey" && activeAccessToken != "" {
			if tokens, err := ad.GetApiTokens(site.URL, activeAccessToken, userID, opt); err == nil {
				if input.ApiToken == "" {
					if preferredToken := preferredApiToken("", tokens); preferredToken != "" {
						_ = db.UpdateAccount(id, map[string]interface{}{"api_token": preferredToken})
					}
				}
				for _, t := range tokens {
					db.CreateAccountToken(id, t.Name, t.Key)
				}
			}
		}
	}()

	ok(w, map[string]interface{}{
		"id":               id,
		"batch":            false,
		"queued":           true,
		"tokenType":        input.CredentialMode,
		"message":          "账号已添加，后台正在同步初始化信息。",
		"usernameDetected": usernameDetected,
		"apiTokenFound":    apiTokenFound,
	})
}

func UpdateAccount(w http.ResponseWriter, r *http.Request) {
	id, valid := parseID(r)
	if !valid {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}

	var fields map[string]interface{}
	if err := parseBody(r, &fields); err != nil {
		fail(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	delete(fields, "id")
	delete(fields, "created_at")
	delete(fields, "skipModelFetch")
	delete(fields, "skip_model_fetch")

	// Merge extra config fields
	account, err := db.GetAccount(id)
	if err != nil {
		fail(w, http.StatusNotFound, "account not found")
		return
	}

	var cfg map[string]interface{}
	if account.ExtraConfig != nil && *account.ExtraConfig != "" {
		json.Unmarshal([]byte(*account.ExtraConfig), &cfg)
	}
	if cfg == nil {
		cfg = make(map[string]interface{})
	}

	cfgModified := false
	if v, ok := fields["proxyUrl"]; ok {
		if s, isStr := v.(string); isStr && s != "" {
			cfg["proxyUrl"] = s
		} else {
			delete(cfg, "proxyUrl")
		}
		delete(fields, "proxyUrl")
		cfgModified = true
	}
	if v, ok := fields["useSystemProxy"]; ok {
		if b, isBool := v.(bool); isBool {
			cfg["useSystemProxy"] = b
		} else {
			delete(cfg, "useSystemProxy")
		}
		delete(fields, "useSystemProxy")
		cfgModified = true
	}
	if v, ok := fields["credentialMode"]; ok {
		if s, isStr := v.(string); isStr && s != "" {
			cfg["credentialMode"] = s
		}
		delete(fields, "credentialMode")
		cfgModified = true
	}
	if v, ok := fields["checkin_credential"]; ok {
		if s, isStr := v.(string); isStr {
			if s != "" {
				cfg["checkin_credential"] = s
			} else {
				delete(cfg, "checkin_credential")
			}
		}
		delete(fields, "checkin_credential")
		cfgModified = true
	}
	if v, ok := fields["platformUserId"]; ok {
		if f, isNum := v.(float64); isNum {
			cfg["platformUserId"] = int64(f)
		}
		delete(fields, "platformUserId")
		cfgModified = true
	}
	if v, ok := fields["refreshToken"]; ok {
		if s, isStr := v.(string); isStr {
			if s != "" {
				sub2apiAuth, _ := cfg["sub2apiAuth"].(map[string]interface{})
				if sub2apiAuth == nil {
					sub2apiAuth = make(map[string]interface{})
				}
				sub2apiAuth["refreshToken"] = s
				cfg["sub2apiAuth"] = sub2apiAuth
			} else {
				delete(cfg, "sub2apiAuth")
			}
		}
		delete(fields, "refreshToken")
		cfgModified = true
	}
	if v, ok := fields["tokenExpiresAt"]; ok {
		if f, isNum := v.(float64); isNum {
			sub2apiAuth, _ := cfg["sub2apiAuth"].(map[string]interface{})
			if sub2apiAuth == nil {
				sub2apiAuth = make(map[string]interface{})
			}
			sub2apiAuth["tokenExpiresAt"] = int64(f)
			cfg["sub2apiAuth"] = sub2apiAuth
		}
		delete(fields, "tokenExpiresAt")
		cfgModified = true
	}

	delete(fields, "skipModelFetch")
	delete(fields, "accessTokens")

	if cfgModified {
		bs, _ := json.Marshal(cfg)
		fields["extra_config"] = string(bs)
	}

	if mode, _ := cfg["credentialMode"].(string); mode == "apikey" {
		if token, ok := fields["access_token"].(string); ok && strings.TrimSpace(token) != "" {
			fields["api_token"] = strings.TrimSpace(token)
		}
		fields["access_token"] = ""
	}

	if err := db.UpdateAccount(id, fields); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	account, _ = db.GetAccount(id)
	ok(w, account)
}

func DeleteAccount(w http.ResponseWriter, r *http.Request) {
	id, valid := parseID(r)
	if !valid {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := db.DeleteAccount(id); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	ok(w, map[string]interface{}{"deleted": true})
}

// ---- Account Tokens ----

func ListAccountTokens(w http.ResponseWriter, r *http.Request) {
	accountID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid account id")
		return
	}

	tokens, err := db.ListAccountTokens(accountID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, tokens)
}

type createTokenInput struct {
	Name  string `json:"name"`
	Token string `json:"token"`
}

func CreateAccountToken(w http.ResponseWriter, r *http.Request) {
	accountID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid account id")
		return
	}

	var input createTokenInput
	if err := parseBody(r, &input); err != nil {
		fail(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	if input.Name == "" || input.Token == "" {
		fail(w, http.StatusBadRequest, "name and token are required")
		return
	}
	if isMaskedTokenValue(input.Token) {
		fail(w, http.StatusBadRequest, "masked token cannot be saved; paste the full token value")
		return
	}

	id, err := db.CreateAccountToken(accountID, input.Name, input.Token)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	ok(w, map[string]interface{}{"id": id})
}

func DeleteAccountToken(w http.ResponseWriter, r *http.Request) {
	id, valid := parseID(r)
	if !valid {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := db.DeleteAccountToken(id); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	ok(w, map[string]interface{}{"deleted": true})
}
