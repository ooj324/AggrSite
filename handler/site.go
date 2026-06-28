package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"metapi/aggrsite/db"
)

func ListSites(w http.ResponseWriter, r *http.Request) {
	sites, err := db.ListSites()
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Aggregate total balance per site
	balances, _ := db.GetSiteBalances()

	type siteWithBalance struct {
		db.Site
		TotalBalance float64 `json:"total_balance"`
	}

	result := make([]siteWithBalance, len(sites))
	for i, s := range sites {
		result[i] = siteWithBalance{
			Site:         s,
			TotalBalance: balances[s.ID],
		}
	}

	ok(w, result)
}

func GetSite(w http.ResponseWriter, r *http.Request) {
	id, valid := parseID(r)
	if !valid {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}

	site, err := db.GetSite(id)
	if err != nil {
		fail(w, http.StatusNotFound, "site not found")
		return
	}
	ok(w, site)
}

func CreateSite(w http.ResponseWriter, r *http.Request) {
	var input db.CreateSiteInput
	if err := parseBody(r, &input); err != nil {
		fail(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	slog.Info("CreateSite input", "input", input)

	if input.Name == "" || input.URL == "" || input.Platform == "" {
		fail(w, http.StatusBadRequest, "name, url, platform are required")
		return
	}

	id, err := db.CreateSite(input)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	site, _ := db.GetSite(id)
	created(w, site)
}

func UpdateSite(w http.ResponseWriter, r *http.Request) {
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
	slog.Info("UpdateSite fields", "id", id, "fields", fields)

	// Prevent updating the id
	delete(fields, "id")
	delete(fields, "created_at")

	// Detect status change for cascade
	newStatus, hasStatus := fields["status"].(string)

	// Get current site to detect actual status change
	var oldStatus string
	if hasStatus {
		currentSite, err := db.GetSite(id)
		if err != nil {
			fail(w, http.StatusNotFound, "site not found")
			return
		}
		oldStatus = currentSite.Status
	}

	if err := db.UpdateSite(id, fields); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Status cascade: propagate to accounts
	if hasStatus && newStatus != oldStatus {
		cascadeSiteStatusChange(id, oldStatus, newStatus)
	}

	site, _ := db.GetSite(id)
	ok(w, site)
}

// cascadeSiteStatusChange propagates site status changes to all associated accounts.
func cascadeSiteStatusChange(siteID int64, oldStatus, newStatus string) {
	site, _ := db.GetSite(siteID)
	siteName := ""
	if site != nil {
		siteName = site.Name
	}

	if newStatus == "disabled" {
		// Disable all accounts under this site
		err := db.UpdateAccountsBySite(siteID, map[string]interface{}{
			"status": "disabled",
		})
		if err != nil {
			slog.Error("Failed to cascade disable accounts", "site_id", siteID, "err", err)
			return
		}
		_ = db.InsertEvent("site", "site disabled",
			fmt.Sprintf("站点 %s 已禁用，关联账号已全部禁用", siteName),
			"warning", &siteID, "site")
		slog.Info("Site disabled, cascaded to accounts", "site_id", siteID, "site_name", siteName)
	} else if newStatus == "active" && oldStatus == "disabled" {
		// Re-enable all disabled accounts under this site
		err := db.UpdateAccountsBySite(siteID, map[string]interface{}{
			"status": "active",
		})
		if err != nil {
			slog.Error("Failed to cascade enable accounts", "site_id", siteID, "err", err)
			return
		}
		_ = db.InsertEvent("site", "site enabled",
			fmt.Sprintf("站点 %s 已启用，关联账号已全部恢复", siteName),
			"info", &siteID, "site")
		slog.Info("Site enabled, cascaded to accounts", "site_id", siteID, "site_name", siteName)
	}
}

func DeleteSite(w http.ResponseWriter, r *http.Request) {
	id, valid := parseID(r)
	if !valid {
		fail(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := db.DeleteSite(id); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}

	ok(w, map[string]interface{}{"deleted": true})
}

// BatchSites handles bulk operations on sites.
func BatchSites(w http.ResponseWriter, r *http.Request) {
	var input struct {
		IDs    []int64 `json:"ids"`
		Action string  `json:"action"`
	}
	if err := parseBody(r, &input); err != nil {
		fail(w, http.StatusBadRequest, "invalid body")
		return
	}

	if len(input.IDs) == 0 {
		fail(w, http.StatusBadRequest, "ids is required")
		return
	}

	var successIDs []int64
	var failedItems []map[string]interface{}

	for _, id := range input.IDs {
		var err error
		switch input.Action {
		case "enable":
			currentSite, getErr := db.GetSite(id)
			if getErr != nil {
				err = getErr
			} else {
				oldStatus := currentSite.Status
				err = db.UpdateSite(id, map[string]interface{}{"status": "active"})
				if err == nil {
					cascadeSiteStatusChange(id, oldStatus, "active")
				}
			}
		case "disable":
			currentSite, getErr := db.GetSite(id)
			if getErr != nil {
				err = getErr
			} else {
				oldStatus := currentSite.Status
				err = db.UpdateSite(id, map[string]interface{}{"status": "disabled"})
				if err == nil {
					cascadeSiteStatusChange(id, oldStatus, "disabled")
				}
			}
		case "delete":
			err = db.DeleteSite(id)
		case "enableSystemProxy":
			err = db.UpdateSite(id, map[string]interface{}{"use_system_proxy": true})
		case "disableSystemProxy":
			err = db.UpdateSite(id, map[string]interface{}{"use_system_proxy": false})
		default:
			fail(w, http.StatusBadRequest, "unsupported action: "+input.Action)
			return
		}

		if err != nil {
			failedItems = append(failedItems, map[string]interface{}{
				"id":      id,
				"message": err.Error(),
			})
		} else {
			successIDs = append(successIDs, id)
		}
	}

	ok(w, map[string]interface{}{
		"successIds":  successIDs,
		"failedItems": failedItems,
	})
}

func DetectSite(w http.ResponseWriter, r *http.Request) {
	var input struct {
		URL string `json:"url"`
	}
	if err := parseBody(r, &input); err != nil {
		fail(w, http.StatusBadRequest, "invalid body")
		return
	}
	url := strings.TrimSpace(input.URL)
	if url == "" {
		fail(w, http.StatusBadRequest, "url is required")
		return
	}
	url = strings.TrimSuffix(url, "/")

	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", url+"/api/status", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	
	resp, err := client.Do(req)
	if err != nil {
		// Cannot connect, return empty or error
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		var data map[string]interface{}
		body, _ := io.ReadAll(resp.Body)
		if json.Unmarshal(body, &data) == nil {
			if sysName, isStr := data["system_name"].(string); isStr && strings.Contains(strings.ToLower(sysName), "new api") {
				ok(w, map[string]interface{}{"platform": "newapi", "url": url})
				return
			}
			if _, hasVer := data["version"]; hasVer {
				ok(w, map[string]interface{}{"platform": "oneapi", "url": url})
				return
			}
		}
	}
	fail(w, http.StatusBadRequest, "Could not detect platform")
}

func PingSite(w http.ResponseWriter, r *http.Request) {
	var input struct {
		URL string `json:"url"`
	}
	if err := parseBody(r, &input); err != nil {
		fail(w, http.StatusBadRequest, "invalid body")
		return
	}
	url := strings.TrimSpace(input.URL)
	if url == "" {
		fail(w, http.StatusBadRequest, "url is required")
		return
	}

	start := time.Now()
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	defer resp.Body.Close()

	ok(w, map[string]interface{}{
		"success":     true,
		"latency_ms":  latency,
		"status_code": resp.StatusCode,
	})
}
