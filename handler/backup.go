package handler

import (
	"encoding/json"
	"io"
	"metapi/aggrsite/db"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type BackupAggrSite struct {
	Version  string       `json:"version"`
	ExportAt string       `json:"export_at"`
	Sites    []db.Site    `json:"sites"`
	Accounts []db.Account `json:"accounts"`
}

func ExportBackup(w http.ResponseWriter, r *http.Request) {
	sites, err := db.ListSites()
	if err != nil {
		fail(w, http.StatusInternalServerError, "Failed to load sites: "+err.Error())
		return
	}
	accounts, err := db.ListAccounts(nil)
	if err != nil {
		fail(w, http.StatusInternalServerError, "Failed to load accounts: "+err.Error())
		return
	}

	backup := BackupAggrSite{
		Version:  "aggrsite-1.0",
		ExportAt: time.Now().UTC().Format(time.RFC3339),
		Sites:    sites,
		Accounts: accounts,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\"aggrsite-backup.json\"")
	json.NewEncoder(w).Encode(backup)
}

func ImportBackup(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10 << 20) // 10 MB limit
	if err != nil {
		fail(w, http.StatusBadRequest, "Failed to parse form")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		fail(w, http.StatusBadRequest, "No file uploaded")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		fail(w, http.StatusInternalServerError, "Failed to read file")
		return
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		fail(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	version, _ := payload["version"].(string)

	var importedSites, importedAccounts int

	if strings.HasPrefix(version, "aggrsite-") {
		// New AggrSite format
		var backup BackupAggrSite
		if err := json.Unmarshal(data, &backup); err != nil {
			fail(w, http.StatusBadRequest, "Failed to parse aggrsite backup: "+err.Error())
			return
		}

		// Ensure sites map
		siteIDMap := make(map[int64]int64) // old id -> new id

		for _, s := range backup.Sites {
			id, _ := db.CreateSite(db.CreateSiteInput{
				Name:                      s.Name,
				URL:                       s.URL,
				Platform:                  s.Platform,
				Status:                    s.Status,
				ProxyURL:                  s.ProxyURL,
				UseSystemProxy:            s.UseSystemProxy,
				ExternalCheckinURL:        s.ExternalCheckinURL,
				ExternalCheckinMethod:     s.ExternalCheckinMethod,
				ExternalCheckinAuthHeader: s.ExternalCheckinAuthHeader,
				ExternalCheckinAuthPrefix: s.ExternalCheckinAuthPrefix,
				ExternalCheckinBody:       s.ExternalCheckinBody,
				CustomHeaders:             s.CustomHeaders,
			})
			if id > 0 {
				siteIDMap[s.ID] = id
				importedSites++
			}
		}

		for _, a := range backup.Accounts {
			newSiteID := siteIDMap[a.SiteID]
			if newSiteID == 0 {
				continue
			}
			id, _ := db.CreateAccount(db.CreateAccountInput{
				SiteID:         newSiteID,
				Username:       nullStr(a.Username),
				AccessToken:    a.AccessToken,
				ApiToken:       nullStr(a.ApiToken),
				CheckinEnabled: a.CheckinEnabled != nil && *a.CheckinEnabled,
			})
			if id > 0 {
				// Restore extra config and balances
				updates := map[string]interface{}{}
				if a.ExtraConfig != nil && *a.ExtraConfig != "" {
					updates["extra_config"] = *a.ExtraConfig
				}
				if a.Balance != nil {
					updates["balance"] = *a.Balance
				}
				if a.BalanceUsed != nil {
					updates["balance_used"] = *a.BalanceUsed
				}
				if a.Quota != nil {
					updates["quota"] = *a.Quota
				}
				if a.Status != nil && *a.Status != "" {
					updates["status"] = *a.Status
				}
				if len(updates) > 0 {
					_ = db.UpdateAccount(id, updates)
				}
				importedAccounts++
			}
		}

	} else if strings.HasPrefix(version, "2.") || payload["accounts"] != nil {
		// Legacy Metapi V2 format
		accountsSection, ok := payload["accounts"].(map[string]interface{})
		if !ok {
			// maybe it is just the accounts array
			if accountsArray, ok := payload["accounts"].([]interface{}); ok {
				accountsSection = map[string]interface{}{"accounts": accountsArray}
			} else {
				fail(w, http.StatusBadRequest, "Legacy backup missing accounts section")
				return
			}
		}

		rawAccounts, _ := accountsSection["accounts"].([]interface{})

		siteUrlToID := make(map[string]int64)

		for _, rawRow := range rawAccounts {
			row, ok := rawRow.(map[string]interface{})
			if !ok {
				continue
			}

			siteUrl, _ := row["site_url"].(string)
			if siteUrl == "" {
				continue
			}
			parsedUrl, _ := url.Parse(strings.TrimSpace(siteUrl))
			if parsedUrl != nil {
				siteUrl = parsedUrl.Scheme + "://" + parsedUrl.Host
			}

			siteType, _ := row["site_type"].(string)
			if siteType == "" {
				siteType = "new-api"
			}

			siteKey := siteType + "::" + siteUrl

			// Create site if not exists
			newSiteID, exists := siteUrlToID[siteKey]
			if !exists {
				siteName, _ := row["site_name"].(string)
				if siteName == "" {
					siteName = siteUrl
				}
				newSiteID, _ = db.CreateSite(db.CreateSiteInput{
					Name:     siteName,
					URL:      siteUrl,
					Platform: siteType,
					Status:   "active",
				})
				siteUrlToID[siteKey] = newSiteID
				importedSites++
			}

			// Extract account
			username, _ := row["username"].(string)
			if username == "" {
				if accInfo, ok := row["account_info"].(map[string]interface{}); ok {
					if u, ok := accInfo["username"].(string); ok {
						username = u
					}
				}
			}

			accessToken, _ := row["access_token"].(string)
			if accessToken == "" {
				if accInfo, ok := row["account_info"].(map[string]interface{}); ok {
					if t, ok := accInfo["access_token"].(string); ok {
						accessToken = t
					}
				}
			}

			apiToken := "" // usually mapped elsewhere in V2, let's keep it empty or try tokens array?
			// Old backups don't have api_token natively in row usually, so we just stick with accessToken

			checkinEnabled := true
			if checkInRaw, ok := row["checkIn"].(map[string]interface{}); ok {
				if e, ok := checkInRaw["enabled"].(bool); ok {
					checkinEnabled = e
				}
			}

			if accessToken != "" || username != "" {
				id, _ := db.CreateAccount(db.CreateAccountInput{
					SiteID:         newSiteID,
					Username:       username,
					AccessToken:    accessToken,
					ApiToken:       apiToken,
					CheckinEnabled: checkinEnabled,
				})
				if id > 0 {
					importedAccounts++
				}
			}
		}

	} else {
		fail(w, http.StatusBadRequest, "Unsupported backup format")
		return
	}

	ok(w, map[string]interface{}{
		"imported_sites":    importedSites,
		"imported_accounts": importedAccounts,
	})
}
