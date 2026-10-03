package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"metapi/aggrsite/db"

	"github.com/go-chi/chi/v5"
)

func mustStatus(t *testing.T, id int64) string {
	t.Helper()
	acc, err := db.GetAccount(id)
	if err != nil {
		t.Fatalf("GetAccount(%d) failed: %v", id, err)
	}
	if acc.Status == nil {
		return "active"
	}
	return strings.TrimSpace(*acc.Status)
}

func mustExtraConfig(t *testing.T, id int64) map[string]interface{} {
	t.Helper()
	acc, err := db.GetAccount(id)
	if err != nil {
		t.Fatalf("GetAccount(%d) failed: %v", id, err)
	}
	cfg := map[string]interface{}{}
	if acc.ExtraConfig != nil && strings.TrimSpace(*acc.ExtraConfig) != "" {
		if err := json.Unmarshal([]byte(*acc.ExtraConfig), &cfg); err != nil {
			t.Fatalf("extra_config not JSON: %v", err)
		}
	}
	return cfg
}

func TestSiteDisableArchivesAndEnableRestores(t *testing.T) {
	siteID := setupVerifyTokenTestDB(t, "https://example.invalid")

	mk := func(username, status string) int64 {
		id, err := db.CreateAccount(db.CreateAccountInput{
			SiteID:      siteID,
			Username:    username,
			AccessToken: "tok-" + username,
			Status:      status,
		})
		if err != nil {
			t.Fatalf("CreateAccount failed: %v", err)
		}
		return id
	}
	activeID := mk("active-user", "active")
	expiredID := mk("expired-user", "expired")
	manualID := mk("manual-disabled", "disabled")

	route := func(r chi.Router) { r.Put("/api/sites/{id}", UpdateSite) }

	// Disable the site.
	rec := callJSONRoute(t, http.MethodPut, "/api/sites/"+strconv.FormatInt(siteID, 10), route, map[string]interface{}{"status": "disabled"})
	responseDataMap(t, rec)

	if s := mustStatus(t, activeID); s != "disabled" {
		t.Fatalf("active account status = %q after disable", s)
	}
	if cfg := mustExtraConfig(t, activeID); cfg[db.ExtraKeyArchivedBySite] != true || cfg[db.ExtraKeyArchivedPrevStatus] != "active" {
		t.Fatalf("active account missing archive snapshot: %v", cfg)
	}
	if cfg := mustExtraConfig(t, expiredID); cfg[db.ExtraKeyArchivedPrevStatus] != "expired" {
		t.Fatalf("expired account wrong snapshot: %v", cfg)
	}
	if cfg := mustExtraConfig(t, manualID); len(cfg) != 0 {
		t.Fatalf("manually disabled account wrongly marked: %v", cfg)
	}

	// Archived accounts are hidden by default; visible with include_archived.
	rec = callJSONRoute(t, http.MethodGet, "/api/accounts", func(r chi.Router) { r.Get("/api/accounts", ListAccounts) }, nil)
	// GET with an empty body: callJSONRoute marshals nil payload — fine for this handler.
	var env map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	visible := env["data"].([]interface{})
	for _, item := range visible {
		m := item.(map[string]interface{})
		if m["id"].(float64) == float64(activeID) || m["id"].(float64) == float64(expiredID) {
			t.Fatalf("archived account leaked into default list: %v", m["id"])
		}
		if disp, ok := m["display"].(map[string]interface{}); !ok || disp["state"] == nil || disp["label"] == nil {
			t.Fatalf("account row missing unified display block: %v", m["id"])
		}
	}
	// Only the manually-disabled account remains visible.
	if len(visible) != 1 {
		t.Fatalf("default list length = %d, want 1", len(visible))
	}

	// Re-enable the site: snapshot restores each original status.
	rec = callJSONRoute(t, http.MethodPut, "/api/sites/"+strconv.FormatInt(siteID, 10), route, map[string]interface{}{"status": "active"})
	responseDataMap(t, rec)

	if s := mustStatus(t, activeID); s != "active" {
		t.Fatalf("active account restored to %q", s)
	}
	if s := mustStatus(t, expiredID); s != "expired" {
		t.Fatalf("expired account restored to %q, want expired", s)
	}
	if s := mustStatus(t, manualID); s != "disabled" {
		t.Fatalf("manually disabled account wrongly revived to %q", s)
	}
	if cfg := mustExtraConfig(t, activeID); len(cfg) != 0 {
		t.Fatalf("archive markers not cleared after restore: %v", cfg)
	}
}

func TestBatchSiteDisableEnableRoundtrip(t *testing.T) {
	siteID := setupVerifyTokenTestDB(t, "https://example.invalid")
	id, err := db.CreateAccount(db.CreateAccountInput{
		SiteID:      siteID,
		Username:    "batch-user",
		AccessToken: "tok-batch",
		Status:      "active",
	})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	route := func(r chi.Router) { r.Post("/api/sites/batch", BatchSites) }

	rec := callJSONRoute(t, http.MethodPost, "/api/sites/batch", route, map[string]interface{}{
		"ids": []int64{siteID}, "action": "disable",
	})
	responseDataMap(t, rec)
	if s := mustStatus(t, id); s != "disabled" {
		t.Fatalf("batch disable did not archive account: %q", s)
	}
	if cfg := mustExtraConfig(t, id); cfg[db.ExtraKeyArchivedBySite] != true {
		t.Fatalf("batch disable missing archive marker: %v", cfg)
	}

	rec = callJSONRoute(t, http.MethodPost, "/api/sites/batch", route, map[string]interface{}{
		"ids": []int64{siteID}, "action": "enable",
	})
	responseDataMap(t, rec)
	if s := mustStatus(t, id); s != "active" {
		t.Fatalf("batch enable did not restore account: %q", s)
	}
}
