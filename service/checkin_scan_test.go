package service

import (
	"metapi/aggrsite/config"
	"metapi/aggrsite/db"
	"os"
	"path/filepath"
	"testing"
)

func setupCheckinScanTestDB(t *testing.T) {
	t.Helper()

	if db.DB != nil {
		_ = db.DB.Close()
		db.DB = nil
	}
	t.Setenv("DB_URL", filepath.Join(t.TempDir(), "checkin-scan.db"))
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

// The checkin scan must keep expired accounts, because "expired" is written
// automatically by a failed checkin: dropping them would turn one auth failure into a
// permanent exit from the scheduler for platforms without managed session refresh.
// Accounts the user turned off (checkin_enabled=0, status=disabled) must stay out.
func TestListCheckinableAccountsCoversExpiredAccounts(t *testing.T) {
	setupCheckinScanTestDB(t)

	newSite := func(name, status string) int64 {
		t.Helper()
		id, err := db.CreateSite(db.CreateSiteInput{Name: name, URL: "https://" + name + ".invalid", Platform: "new-api", Status: status})
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

	site := newSite("checkin-site", "active")

	// CreateAccount normalizes an empty status to "active", so write the raw status
	// directly to cover the NULL / empty / mixed-case values the query normalizes.
	rawStatus := func(username, status string, checkinEnabled bool) int64 {
		t.Helper()
		id := newAccount(site, username, "active", checkinEnabled)
		var value interface{}
		if status != "<nil>" {
			value = status
		}
		if _, err := db.Exec(`UPDATE accounts SET status = ? WHERE id = ?`, value, id); err != nil {
			t.Fatalf("forcing status %q on %s failed: %v", status, username, err)
		}
		return id
	}

	wantIDs := map[int64]string{
		newAccount(site, "active", "active", true):   "active account",
		newAccount(site, "expired", "expired", true): "expired account",
		rawStatus("blank", "", true):                 "account with an empty status",
		rawStatus("null-status", "<nil>", true):      "account with a NULL status",
		rawStatus("mixed-case", " Expired ", true):   "expired account with padding and mixed case",
	}
	skipIDs := map[int64]string{
		newAccount(site, "checkin-off", "active", false):    "account with checkin disabled",
		newAccount(site, "acct-disabled", "disabled", true): "disabled account",
		rawStatus("expired-off", "expired", false):          "expired account the user turned off",
	}

	rows, err := db.ListCheckinableAccounts()
	if err != nil {
		t.Fatalf("ListCheckinableAccounts failed: %v", err)
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

// The balance scan keeps expired accounts for the same reason, and additionally ignores
// checkin_enabled: balance-only accounts still need refreshing. A successful refresh is
// what flips an expired account back to active.
func TestListBalanceRefreshableAccountsCoversExpiredAccounts(t *testing.T) {
	setupCheckinScanTestDB(t)

	site, err := db.CreateSite(db.CreateSiteInput{Name: "balance-site", URL: "https://balance.invalid", Platform: "new-api", Status: "active"})
	if err != nil {
		t.Fatalf("CreateSite failed: %v", err)
	}
	newAccount := func(username, status string, checkinEnabled bool) int64 {
		t.Helper()
		id, err := db.CreateAccount(db.CreateAccountInput{
			SiteID:         site,
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

	wantIDs := map[int64]string{
		newAccount("active", "active", true):        "active account",
		newAccount("expired", "expired", true):      "expired account",
		newAccount("balance-only", "active", false): "balance-only account",
		newAccount("expired-off", "expired", false): "expired balance-only account",
	}
	skipIDs := map[int64]string{
		newAccount("acct-disabled", "disabled", true): "disabled account",
	}

	rows, err := db.ListBalanceRefreshableAccounts()
	if err != nil {
		t.Fatalf("ListBalanceRefreshableAccounts failed: %v", err)
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
