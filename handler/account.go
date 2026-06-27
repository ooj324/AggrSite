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

func getRequestOption(site *db.Site) *platform.RequestOption {
	return &platform.RequestOption{
		ProxyURL:       site.ProxyURL,
		UseSystemProxy: site.UseSystemProxy,
		CustomHeaders:  site.CustomHeaders,
	}
}

func VerifyToken(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SiteID         int64  `json:"siteId"`
		AccessToken    string `json:"accessToken"`
		PlatformUserID int64  `json:"platformUserId"`
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

	// Mode handling
	credentialMode := r.URL.Query().Get("credentialMode")
	if credentialMode == "" {
		credentialMode = "session"
	}

	if credentialMode == "apikey" {
		// API Key mode: just test if we can fetch models
		models, err := ad.GetModels(site.URL, input.AccessToken, input.PlatformUserID, opt)
		if err != nil {
			errStr := err.Error()
			shieldBlocked := strings.Contains(errStr, "acw_sc__v2") || strings.Contains(errStr, "var arg1") || strings.Contains(errStr, "challenge") || strings.Contains(errStr, "cloudflare") || strings.Contains(errStr, "invalid character")
			if shieldBlocked {
				ok(w, map[string]interface{}{"shieldBlocked": true, "message": "被反爬或防火墙拦截"})
				return
			}
			fail(w, http.StatusInternalServerError, errStr)
			return
		}
		ok(w, map[string]interface{}{
			"tokenType":  "apikey",
			"modelCount": len(models),
			"models":     models,
		})
		return
	}

	res, err := ad.VerifyToken(site.URL, input.AccessToken, input.PlatformUserID, opt)
	if err != nil {
		errStr := err.Error()
		// Detect needsUserId or shield
		needsUserId := strings.Contains(errStr, "New-API-User") || strings.Contains(errStr, "user id required")
		shieldBlocked := strings.Contains(errStr, "acw_sc__v2") || strings.Contains(errStr, "var arg1") || strings.Contains(errStr, "challenge") || strings.Contains(errStr, "cloudflare") || strings.Contains(errStr, "invalid character")
		if needsUserId || shieldBlocked {
			ok(w, map[string]interface{}{
				"needsUserId":   needsUserId,
				"shieldBlocked": shieldBlocked,
				"message":       errStr,
			})
			return
		}
		fail(w, http.StatusInternalServerError, errStr)
		return
	}

	ok(w, res)
}

func RebindSession(w http.ResponseWriter, r *http.Request) {
	id, valid := parseID(r)
	if !valid {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}

	var input struct {
		AccessToken    string `json:"accessToken"`
		PlatformUserID *int64 `json:"platformUserId"`
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
	var input db.CreateAccountInput
	if err := parseBody(r, &input); err != nil {
		fail(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	if input.SiteID == 0 {
		fail(w, http.StatusBadRequest, "site_id is required")
		return
	}

	if input.AccessToken == "" && input.ApiToken == "" {
		fail(w, http.StatusBadRequest, "access_token or api_token is required")
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

	// If missing ApiToken and it's a session token, try to fetch it
	if input.ApiToken == "" && input.AccessToken != "" && input.CredentialMode != "apikey" {
		userID := int64(0)
		if input.PlatformUserID != nil {
			userID = *input.PlatformUserID
		}
		if token, err := ad.GetApiToken(site.URL, input.AccessToken, userID, opt); err == nil {
			input.ApiToken = token
		}
	}

	id, err := db.CreateAccount(input)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Trigger async sync logic
	go func() {
		userID := int64(0)
		if input.PlatformUserID != nil {
			userID = *input.PlatformUserID
		}
		
		// sync balance
		if input.CredentialMode != "apikey" && input.AccessToken != "" {
			service.RefreshBalance(id)
		}
		
		// sync tokens
		if input.CredentialMode != "apikey" && input.AccessToken != "" {
			if tokens, err := ad.GetApiTokens(site.URL, input.AccessToken, userID, opt); err == nil {
				for _, t := range tokens {
					db.CreateAccountToken(id, t.Name, t.Key)
				}
			}
		}
	}()

	account, _ := db.GetAccount(id)
	created(w, account)
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

	if err := db.UpdateAccount(id, fields); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	account, _ := db.GetAccount(id)
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
