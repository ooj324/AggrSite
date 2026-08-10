package service

import (
	"metapi/aggrsite/config"
	"metapi/aggrsite/db"
	"os"
	"path/filepath"
	"testing"
)

func setupManagedScanTestDB(t *testing.T) {
	t.Helper()

	if db.DB != nil {
		_ = db.DB.Close()
		db.DB = nil
	}
	t.Setenv("DB_URL", filepath.Join(t.TempDir(), "managed-scan.db"))
	config.Init()
	db.Init()
	t.Cleanup(func() {
		if db.DB != nil {
			_ = db.DB.Close()
			db.DB = nil
		}
		_ = os.Remove(config.C.DBUrl)
	})
}

// The managed refresh scan must cover every account whose session we keep alive,
// including balance-only accounts (checkin disabled) and expired ones, since those
// are exactly the accounts a refresh can recover.
func TestListAccountsWithSiteByPlatformsCoversManagedAccounts(t *testing.T) {
	setupManagedScanTestDB(t)

	newSite := func(name, platform, status string) int64 {
		t.Helper()
		id, err := db.CreateSite(db.CreateSiteInput{Name: name, URL: "https://" + name + ".invalid", Platform: platform, Status: status})
		if err != nil {
			t.Fatalf("CreateSite(%s) failed: %v", name, err)
		}
		return id
	}
	newAccount := func(siteID int64, username, status string, checkinEnabled bool) int64 {
		t.Helper()
		id, err := db.CreateAccount(db.CreateAccountInput{
			SiteID:         siteID,
			Username:       username,
			AccessToken:    "token-" + username,
			CheckinEnabled: checkinEnabled,
			Status:         status,
		})
		if err != nil {
			t.Fatalf("CreateAccount(%s) failed: %v", username, err)
		}
		return id
	}

	sub2apiSite := newSite("sub2api-site", "sub2api", "active")
	newApiV1Site := newSite("newapiv1-site", "new-api-v1", "active")
	plainSite := newSite("plain-site", "new-api", "active")
	disabledSite := newSite("disabled-site", "sub2api", "disabled")

	wantIDs := map[int64]string{
		newAccount(sub2apiSite, "checkin-on", "active", true):   "active sub2api account",
		newAccount(sub2apiSite, "checkin-off", "active", false): "balance-only sub2api account",
		newAccount(sub2apiSite, "expired", "expired", true):     "expired sub2api account",
		newAccount(newApiV1Site, "newapiv1", "active", true):    "active new-api-v1 account",
	}
	skipIDs := map[int64]string{
		newAccount(plainSite, "plain", "active", true):                "non-managed platform",
		newAccount(disabledSite, "on-disabled-site", "active", true):  "account on a disabled site",
		newAccount(sub2apiSite, "account-disabled", "disabled", true): "disabled account",
	}

	rows, err := db.ListAccountsWithSiteByPlatforms(ManagedSessionPlatforms())
	if err != nil {
		t.Fatalf("ListAccountsWithSiteByPlatforms failed: %v", err)
	}

	got := make(map[int64]bool, len(rows))
	for _, row := range rows {
		got[row.ID] = true
	}
	for id, description := range wantIDs {
		if !got[id] {
			t.Fatalf("scan is missing %s (id=%d)", description, id)
		}
	}
	for id, description := range skipIDs {
		if got[id] {
			t.Fatalf("scan must not include %s (id=%d)", description, id)
		}
	}
	if len(rows) != len(wantIDs) {
		t.Fatalf("scan returned %d rows, want %d", len(rows), len(wantIDs))
	}
}

func TestListAccountsWithSiteByPlatformsIgnoresEmptyInput(t *testing.T) {
	setupManagedScanTestDB(t)

	rows, err := db.ListAccountsWithSiteByPlatforms([]string{"", "   "})
	if err != nil {
		t.Fatalf("ListAccountsWithSiteByPlatforms failed: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected no rows, got %d", len(rows))
	}
}
