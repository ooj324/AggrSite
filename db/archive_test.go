package db

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"metapi/aggrsite/config"
)

func setupArchiveTestDB(t *testing.T) int64 {
	t.Helper()

	if DB != nil {
		_ = DB.Close()
		DB = nil
	}
	t.Setenv("DB_URL", filepath.Join(t.TempDir(), "aggrsite-archive-test.db"))
	config.Init()
	Init()

	siteID, err := CreateSite(CreateSiteInput{
		Name:     "Archive Site",
		URL:      "https://archive.invalid",
		Platform: "new-api",
		Status:   "active",
	})
	if err != nil {
		t.Fatalf("CreateSite failed: %v", err)
	}
	t.Cleanup(func() {
		if DB != nil {
			_ = DB.Close()
			DB = nil
		}
		_ = os.Remove(config.C.DBUrl)
	})
	return siteID
}

func mustCreateAccount(t *testing.T, siteID int64, username, status string) int64 {
	t.Helper()
	id, err := CreateAccount(CreateAccountInput{
		SiteID:      siteID,
		Username:    username,
		AccessToken: "tok-" + username,
		Status:      status,
	})
	if err != nil {
		t.Fatalf("CreateAccount(%s) failed: %v", username, err)
	}
	return id
}

func accountStatusAndConfig(t *testing.T, id int64) (string, map[string]interface{}) {
	t.Helper()
	acc, err := GetAccount(id)
	if err != nil {
		t.Fatalf("GetAccount(%d) failed: %v", id, err)
	}
	cfg := map[string]interface{}{}
	if acc.ExtraConfig != nil && strings.TrimSpace(*acc.ExtraConfig) != "" {
		if err := json.Unmarshal([]byte(*acc.ExtraConfig), &cfg); err != nil {
			t.Fatalf("extra_config of %d is not JSON: %v", id, err)
		}
	}
	return NormalizeAccountStatus(acc.Status), cfg
}

func TestArchiveAccountsBySiteSnapshotsStatus(t *testing.T) {
	siteID := setupArchiveTestDB(t)
	activeID := mustCreateAccount(t, siteID, "active-user", "active")
	expiredID := mustCreateAccount(t, siteID, "expired-user", "expired")
	manualDisabledID := mustCreateAccount(t, siteID, "manual-off", "disabled")

	count, err := ArchiveAccountsBySite(siteID)
	if err != nil {
		t.Fatalf("ArchiveAccountsBySite failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("archived count = %d, want 2", count)
	}

	status, cfg := accountStatusAndConfig(t, activeID)
	if status != "disabled" || cfg[ExtraKeyArchivedBySite] != true || cfg[ExtraKeyArchivedPrevStatus] != "active" {
		t.Fatalf("active account not archived correctly: status=%q cfg=%v", status, cfg)
	}
	status, cfg = accountStatusAndConfig(t, expiredID)
	if status != "disabled" || cfg[ExtraKeyArchivedBySite] != true || cfg[ExtraKeyArchivedPrevStatus] != "expired" {
		t.Fatalf("expired account not archived correctly: status=%q cfg=%v", status, cfg)
	}
	// Manual disable must be untouched and carry no marker.
	status, cfg = accountStatusAndConfig(t, manualDisabledID)
	if status != "disabled" {
		t.Fatalf("manually disabled account changed: status=%q", status)
	}
	if _, marked := cfg[ExtraKeyArchivedBySite]; marked {
		t.Fatalf("manually disabled account unexpectedly marked archived: %v", cfg)
	}
}

func TestArchiveThenRestoreIsLossless(t *testing.T) {
	siteID := setupArchiveTestDB(t)
	activeID := mustCreateAccount(t, siteID, "active-user", "active")
	expiredID := mustCreateAccount(t, siteID, "expired-user", "expired")
	manualDisabledID := mustCreateAccount(t, siteID, "manual-off", "disabled")

	if _, err := ArchiveAccountsBySite(siteID); err != nil {
		t.Fatalf("archive failed: %v", err)
	}
	restored, err := RestoreAccountsBySite(siteID)
	if err != nil {
		t.Fatalf("RestoreAccountsBySite failed: %v", err)
	}
	if restored != 2 {
		t.Fatalf("restored count = %d, want 2", restored)
	}

	status, cfg := accountStatusAndConfig(t, activeID)
	if status != "active" {
		t.Fatalf("active account restored to %q", status)
	}
	if _, exists := cfg[ExtraKeyArchivedBySite]; exists {
		t.Fatalf("archive marker not cleared: %v", cfg)
	}
	status, _ = accountStatusAndConfig(t, expiredID)
	if status != "expired" {
		t.Fatalf("expired account restored to %q, want expired (no damage)", status)
	}
	status, _ = accountStatusAndConfig(t, manualDisabledID)
	if status != "disabled" {
		t.Fatalf("manually disabled account was revived by restore: %q", status)
	}
}

func TestRestoreDoesNotDoubleApply(t *testing.T) {
	siteID := setupArchiveTestDB(t)
	id := mustCreateAccount(t, siteID, "user", "active")

	if _, err := ArchiveAccountsBySite(siteID); err != nil {
		t.Fatalf("archive failed: %v", err)
	}
	if _, err := RestoreAccountsBySite(siteID); err != nil {
		t.Fatalf("restore 1 failed: %v", err)
	}
	// Second restore must be a no-op: markers were cleared.
	restored, err := RestoreAccountsBySite(siteID)
	if err != nil {
		t.Fatalf("restore 2 failed: %v", err)
	}
	if restored != 0 {
		t.Fatalf("second restore touched %d accounts, want 0", restored)
	}
	status, _ := accountStatusAndConfig(t, id)
	if status != "active" {
		t.Fatalf("status = %q after double restore", status)
	}
}

func TestArchivePreservesExtraConfig(t *testing.T) {
	siteID := setupArchiveTestDB(t)
	id := mustCreateAccount(t, siteID, "user", "active")
	if err := UpdateAccount(id, map[string]interface{}{
		"extra_config": `{"credentialMode":"apikey","proxyUrl":"http://127.0.0.1:7890"}`,
	}); err != nil {
		t.Fatalf("seed extra_config failed: %v", err)
	}

	if _, err := ArchiveAccountsBySite(siteID); err != nil {
		t.Fatalf("archive failed: %v", err)
	}
	if _, err := RestoreAccountsBySite(siteID); err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	_, cfg := accountStatusAndConfig(t, id)
	if cfg["credentialMode"] != "apikey" || cfg["proxyUrl"] != "http://127.0.0.1:7890" {
		t.Fatalf("extra_config keys lost across archive round trip: %v", cfg)
	}
}

func TestSetAccountStatusUserDrivenClearsArchiveMarker(t *testing.T) {
	siteID := setupArchiveTestDB(t)
	id := mustCreateAccount(t, siteID, "user", "active")
	if _, err := ArchiveAccountsBySite(siteID); err != nil {
		t.Fatalf("archive failed: %v", err)
	}

	// User manually re-enables the account while the site is still disabled.
	if err := SetAccountStatusUserDriven(id, "active"); err != nil {
		t.Fatalf("SetAccountStatusUserDriven failed: %v", err)
	}
	_, cfg := accountStatusAndConfig(t, id)
	if _, marked := cfg[ExtraKeyArchivedBySite]; marked {
		t.Fatalf("archive marker survived user-driven status change: %v", cfg)
	}

	// Later site restore must not revert/excite this now-unmarked account.
	if _, err := RestoreAccountsBySite(siteID); err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	status, _ := accountStatusAndConfig(t, id)
	if status != "active" {
		t.Fatalf("status = %q after site restore, want active", status)
	}
}

func TestComputeAccountDisplay(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	cases := []struct {
		name        string
		status      *string
		extraConfig *string
		wantState   string
		wantLabel   string
	}{
		{name: "nil status is healthy", status: nil, extraConfig: nil, wantState: DisplayStateHealthy, wantLabel: "正常"},
		{name: "active healthy", status: strPtr("active"), extraConfig: nil, wantState: DisplayStateHealthy, wantLabel: "正常"},
		{name: "expired is abnormal/令牌失效", status: strPtr("expired"), extraConfig: nil, wantState: DisplayStateAbnormal, wantLabel: "令牌失效"},
		{name: "manual disabled", status: strPtr("disabled"), extraConfig: nil, wantState: DisplayStateDisabled, wantLabel: "禁用"},
		{name: "archived wins over disabled", status: strPtr("disabled"),
			extraConfig: strPtr(`{"archivedBySite":true,"archivedPrevStatus":"active"}`),
			wantState:   DisplayStateArchived, wantLabel: "已归档"},
		{name: "runtime degraded on active", status: strPtr("active"),
			extraConfig: strPtr(`{"runtimeHealth":{"state":"degraded","reason":"签到需要人机验证","source":"checkin"}}`),
			wantState:   DisplayStateDegraded, wantLabel: "需关注"},
		{name: "runtime unhealthy maps to abnormal", status: strPtr("active"),
			extraConfig: strPtr(`{"runtimeHealth":{"state":"unhealthy","reason":"boom","source":"balance"}}`),
			wantState:   DisplayStateAbnormal, wantLabel: "异常"},
		{name: "disabled status beats runtime healthy", status: strPtr("disabled"),
			extraConfig: strPtr(`{"runtimeHealth":{"state":"healthy","reason":"余额刷新成功","source":"balance"}}`),
			wantState:   DisplayStateDisabled, wantLabel: "禁用"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeAccountDisplay(tc.status, tc.extraConfig)
			if got.State != tc.wantState || got.Label != tc.wantLabel {
				t.Fatalf("display = %+v, want state=%q label=%q", got, tc.wantState, tc.wantLabel)
			}
			if got.Reason == "" {
				t.Fatalf("display reason must never be empty: %+v", got)
			}
		})
	}

	// Reason normalization (prefix stripping moved from the frontend).
	d := ComputeAccountDisplay(strPtr("expired"), strPtr(`{"runtimeHealth":{"state":"unhealthy","reason":"failed to fetch balance: invalid token","source":"balance"}}`))
	if d.Reason != "invalid token" {
		t.Fatalf("reason not normalized: %q", d.Reason)
	}
	if d.Source != "余额" {
		t.Fatalf("source label = %q, want 余额", d.Source)
	}
}
