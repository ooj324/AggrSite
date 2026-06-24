package handler

import (
	"encoding/json"
	"log/slog"
	"metapi/aggrsite/db"
	"metapi/aggrsite/platform"
	"metapi/aggrsite/service"
	"net/http"
	"time"
)

type LoginAccountInput struct {
	SiteID   int64  `json:"site_id"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func LoginAccount(w http.ResponseWriter, r *http.Request) {
	var input LoginAccountInput
	if err := parseBody(r, &input); err != nil {
		fail(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}

	if input.SiteID == 0 || input.Username == "" || input.Password == "" {
		fail(w, http.StatusBadRequest, "site_id, username, password are required")
		return
	}

	site, err := db.GetSite(input.SiteID)
	if err != nil {
		fail(w, http.StatusNotFound, "site not found")
		return
	}

	adapter := platform.GetAdapter(site.Platform)
	if adapter == nil {
		fail(w, http.StatusBadRequest, "unsupported platform: "+site.Platform)
		return
	}

	slog.Info("LoginAccount: attempting login", "site_id", site.ID, "username", input.Username)
	opt := &platform.RequestOption{
		ProxyURL:       site.ProxyURL,
		UseSystemProxy: site.UseSystemProxy,
		CustomHeaders:  site.CustomHeaders,
	}

	loginResult, err := adapter.Login(site.URL, input.Username, input.Password, opt)
	if err != nil {
		fail(w, http.StatusInternalServerError, "login error: "+err.Error())
		return
	}

	if !loginResult.Success {
		fail(w, http.StatusBadRequest, "login failed: "+loginResult.Message)
		return
	}

	// Try auto fetch api token
	var apiToken string
	if fetchedApiToken, err := adapter.GetApiToken(site.URL, loginResult.AccessToken, 0, opt); err == nil && fetchedApiToken != "" {
		apiToken = fetchedApiToken
	}

	existing, err := db.GetAccountBySiteAndUsername(site.ID, input.Username)
	
	// Create extra_config
	var cfg map[string]interface{}
	if existing != nil && existing.ExtraConfig != nil && *existing.ExtraConfig != "" {
		_ = json.Unmarshal([]byte(*existing.ExtraConfig), &cfg)
	}
	if cfg == nil {
		cfg = make(map[string]interface{})
	}
	cfg["credentialMode"] = "session"
	cfg["autoRelogin"] = map[string]interface{}{
		"username":       input.Username,
		"passwordCipher": service.EncryptPassword(input.Password),
		"updatedAt":      time.Now().UTC().Format(time.RFC3339),
	}
	
	cfgBytes, _ := json.Marshal(cfg)
	cfgString := string(cfgBytes)

	var accountID int64
	if existing != nil {
		accountID = existing.ID
		updates := map[string]interface{}{
			"access_token":    loginResult.AccessToken,
			"checkin_enabled": 1,
			"status":          "active",
			"extra_config":    cfgString,
		}
		// In sqlite, checkin_enabled should be 1
		if db.DB.DriverName() == "postgres" {
			updates["checkin_enabled"] = true
		}
		if apiToken != "" {
			updates["api_token"] = apiToken
		}
		if err := db.UpdateAccount(accountID, updates); err != nil {
			fail(w, http.StatusInternalServerError, "failed to update account: "+err.Error())
			return
		}
	} else {
		// Create new account
		createInput := db.CreateAccountInput{
			SiteID:         site.ID,
			Username:       input.Username,
			AccessToken:    loginResult.AccessToken,
			ApiToken:       apiToken,
			CheckinEnabled: true,
		}
		id, err := db.CreateAccount(createInput)
		if err != nil {
			fail(w, http.StatusInternalServerError, "failed to create account: "+err.Error())
			return
		}
		accountID = id
		// Update extra_config immediately after create
		_ = db.UpdateAccount(accountID, map[string]interface{}{
			"extra_config": cfgString,
		})
	}

	// Trigger balance refresh asynchronously
	go func() {
		_, _ = service.RefreshBalance(accountID)
	}()

	account, _ := db.GetAccount(accountID)
	ok(w, map[string]interface{}{
		"account":        account,
		"api_token_found": apiToken != "",
	})
}
