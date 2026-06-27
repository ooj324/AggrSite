package db

import (
	"encoding/json"
	"time"
)

// ---- helpers ----

func TimeNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// ---- Site ----

const siteColumns = `id, name, url, platform, status, created_at, updated_at, is_pinned, sort_order, proxy_url, use_system_proxy, custom_headers, external_checkin_url`

type Site struct {
	ID                 int64          `db:"id" json:"id"`
	Name               string         `db:"name" json:"name"`
	URL                string         `db:"url" json:"url"`
	Platform           string         `db:"platform" json:"platform"`
	Status             string         `db:"status" json:"status"`
	CreatedAt          *string `db:"created_at" json:"created_at"`
	UpdatedAt          *string `db:"updated_at" json:"updated_at"`
	IsPinned           *bool   `db:"is_pinned" json:"is_pinned"`
	SortOrder          *int64  `db:"sort_order" json:"sort_order"`
	ProxyURL           *string `db:"proxy_url" json:"proxy_url"`
	UseSystemProxy     *bool   `db:"use_system_proxy" json:"use_system_proxy"`
	CustomHeaders      *string `db:"custom_headers" json:"custom_headers"`
	ExternalCheckinURL *string `db:"external_checkin_url" json:"external_checkin_url"`
}

func ListSites() ([]Site, error) {
	var sites []Site
	err := Select(&sites, `SELECT `+siteColumns+` FROM sites ORDER BY sort_order ASC, id ASC`)
	return sites, err
}

func GetSite(id int64) (*Site, error) {
	var s Site
	err := Get(&s, `SELECT `+siteColumns+` FROM sites WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

type CreateSiteInput struct {
	Name               string  `json:"name"`
	URL                string  `json:"url"`
	Platform           string  `json:"platform"`
	Status             string  `json:"status"`
	ProxyURL           *string `json:"proxy_url"`
	UseSystemProxy     *bool   `json:"use_system_proxy"`
	ExternalCheckinURL *string `json:"external_checkin_url"`
	CustomHeaders      *string `json:"custom_headers"`
}

func CreateSite(in CreateSiteInput) (int64, error) {
	now := TimeNow()
	status := in.Status
	if status == "" {
		status = "active"
	}
	
	// Default use_system_proxy
	useSystemProxyVal := interface{}(0)
	if driverName == "postgres" {
		useSystemProxyVal = false
	}
	if in.UseSystemProxy != nil && *in.UseSystemProxy {
		if driverName == "postgres" {
			useSystemProxyVal = true
		} else {
			useSystemProxyVal = 1
		}
	}
	
	query := `INSERT INTO sites (name, url, platform, status, proxy_url, use_system_proxy, external_checkin_url, custom_headers, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	
	if driverName == "postgres" {
		var id int64
		err := DB.QueryRowx(DB.Rebind(query+` RETURNING id`),
			in.Name, in.URL, in.Platform, status, in.ProxyURL, useSystemProxyVal, in.ExternalCheckinURL, in.CustomHeaders, now, now).Scan(&id)
		return id, err
	}
	
	res, err := Exec(query, in.Name, in.URL, in.Platform, status, in.ProxyURL, useSystemProxyVal, in.ExternalCheckinURL, in.CustomHeaders, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UpdateSite(id int64, in map[string]interface{}) error {
	in["updated_at"] = TimeNow()
	query := "UPDATE sites SET "
	args := []interface{}{}
	i := 0
	for k, v := range in {
		safeKey := ""
		for _, c := range k {
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
				safeKey += string(c)
			}
		}
		if safeKey == "" {
			continue
		}
		
		if i > 0 {
			query += ", "
		}
		query += safeKey + " = ?"
		args = append(args, v)
		i++
	}
	
	if i == 0 {
		return nil
	}
	
	query += " WHERE id = ?"
	args = append(args, id)
	_, err := Exec(query, args...)
	return err
}

func DeleteSite(id int64) error {
	_, err := Exec(`DELETE FROM sites WHERE id = ?`, id)
	return err
}

// UpdateAccountsBySite batch-updates all accounts belonging to a site.
func UpdateAccountsBySite(siteID int64, fields map[string]interface{}) error {
	fields["updated_at"] = TimeNow()
	query := "UPDATE accounts SET "
	args := []interface{}{}
	i := 0
	for k, v := range fields {
		safeKey := ""
		for _, c := range k {
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
				safeKey += string(c)
			}
		}
		if safeKey == "" {
			continue
		}
		if i > 0 {
			query += ", "
		}
		query += safeKey + " = ?"
		args = append(args, v)
		i++
	}
	if i == 0 {
		return nil
	}
	query += " WHERE site_id = ?"
	args = append(args, siteID)
	_, err := Exec(query, args...)
	return err
}

// GetSiteBalances returns a map of site_id -> total_balance for all sites.
func GetSiteBalances() (map[int64]float64, error) {
	type row struct {
		SiteID  int64   `db:"site_id"`
		Balance float64 `db:"total_balance"`
	}
	var rows []row
	err := Select(&rows, `SELECT site_id, COALESCE(SUM(balance), 0) AS total_balance FROM accounts WHERE status = 'active' GROUP BY site_id`)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]float64)
	for _, r := range rows {
		result[r.SiteID] = r.Balance
	}
	return result, nil
}

// ---- Account ----

const accountColumns = `id, site_id, username, access_token, api_token, balance, balance_used, quota, status, checkin_enabled, last_checkin_at, last_balance_refresh, extra_config, created_at, updated_at, is_pinned, sort_order`

type Account struct {
	ID                 int64           `db:"id" json:"id"`
	SiteID             int64           `db:"site_id" json:"site_id"`
	Username           *string  `db:"username" json:"username"`
	AccessToken        string   `db:"access_token" json:"access_token"`
	ApiToken           *string  `db:"api_token" json:"api_token"`
	Balance            *float64 `db:"balance" json:"balance"`
	BalanceUsed        *float64 `db:"balance_used" json:"balance_used"`
	Quota              *float64 `db:"quota" json:"quota"`
	Status             *string  `db:"status" json:"status"`
	CheckinEnabled     *bool    `db:"checkin_enabled" json:"checkin_enabled"`
	LastCheckinAt      *string  `db:"last_checkin_at" json:"last_checkin_at"`
	LastBalanceRefresh *string  `db:"last_balance_refresh" json:"last_balance_refresh"`
	ExtraConfig        *string  `db:"extra_config" json:"extra_config"`
	CreatedAt          *string  `db:"created_at" json:"created_at"`
	UpdatedAt          *string  `db:"updated_at" json:"updated_at"`
	IsPinned           *bool    `db:"is_pinned" json:"is_pinned"`
	SortOrder          *int64   `db:"sort_order" json:"sort_order"`
}

func ListAccounts(siteID *int64) ([]Account, error) {
	var accounts []Account
	if siteID != nil {
		err := Select(&accounts, `SELECT `+accountColumns+` FROM accounts WHERE site_id = ? ORDER BY sort_order ASC, id ASC`, *siteID)
		return accounts, err
	}
	err := Select(&accounts, `SELECT `+accountColumns+` FROM accounts ORDER BY sort_order ASC, id ASC`)
	return accounts, err
}

func GetAccount(id int64) (*Account, error) {
	var a Account
	err := Get(&a, `SELECT `+accountColumns+` FROM accounts WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func GetAccountBySiteAndUsername(siteID int64, username string) (*Account, error) {
	var a Account
	err := Get(&a, `SELECT `+accountColumns+` FROM accounts WHERE site_id = ? AND username = ?`, siteID, username)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

type CreateAccountInput struct {
	SiteID         int64   `json:"site_id"`
	Username       string  `json:"username"`
	AccessToken    string  `json:"access_token"`
	ApiToken       string  `json:"api_token"`
	CheckinEnabled bool    `json:"checkin_enabled"`
	PlatformUserID *int64  `json:"platformUserId"`
	CredentialMode string  `json:"credentialMode"`
	SkipModelFetch bool    `json:"skipModelFetch"`
}

func CreateAccount(in CreateAccountInput) (int64, error) {
	now := TimeNow()
	enabled := 1
	if !in.CheckinEnabled {
		enabled = 0
	}
	
	extraConfig := map[string]interface{}{}
	if in.CredentialMode != "" {
		extraConfig["credentialMode"] = in.CredentialMode
	}
	if in.PlatformUserID != nil {
		extraConfig["platformUserId"] = *in.PlatformUserID
	}
	var extraConfigStr *string
	if len(extraConfig) > 0 {
		bs, _ := json.Marshal(extraConfig)
		s := string(bs)
		extraConfigStr = &s
	}

	if driverName == "postgres" {
		var id int64
		err := DB.QueryRowx(DB.Rebind(`INSERT INTO accounts (site_id, username, access_token, api_token, checkin_enabled, status, extra_config, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 'active', ?, ?, ?) RETURNING id`),
			in.SiteID, nilIfEmpty(in.Username), in.AccessToken, nilIfEmpty(in.ApiToken), in.CheckinEnabled, extraConfigStr, now, now).Scan(&id)
		return id, err
	}
	res, err := Exec(`INSERT INTO accounts (site_id, username, access_token, api_token, checkin_enabled, status, extra_config, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 'active', ?, ?, ?)`,
		in.SiteID, nilIfEmpty(in.Username), in.AccessToken, nilIfEmpty(in.ApiToken), enabled, extraConfigStr, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UpdateAccount(id int64, fields map[string]interface{}) error {
	fields["updated_at"] = TimeNow()
	query := "UPDATE accounts SET "
	args := []interface{}{}
	i := 0
	for k, v := range fields {
		// Basic sanitization of keys to prevent SQL injection
		safeKey := ""
		for _, c := range k {
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
				safeKey += string(c)
			}
		}
		if safeKey == "" {
			continue
		}
		
		if i > 0 {
			query += ", "
		}
		query += safeKey + " = ?"
		args = append(args, v)
		i++
	}
	
	if i == 0 {
		return nil
	}
	
	query += " WHERE id = ?"
	args = append(args, id)
	_, err := Exec(query, args...)
	return err
}

func DeleteAccount(id int64) error {
	_, err := Exec(`DELETE FROM accounts WHERE id = ?`, id)
	return err
}

// AccountWithSiteName is used for listing accounts with their site info.
type AccountWithSiteName struct {
	Account
	SiteName     string `db:"site_name" json:"site_name"`
	SitePlatform string `db:"site_platform" json:"site_platform"`
	SiteURL      string `db:"site_url" json:"site_url"`
}

func ListAccountsWithSites(siteID *int64) ([]AccountWithSiteName, error) {
	var accounts []AccountWithSiteName
	baseQuery := `SELECT ` + accountColumns + `, s.name AS site_name, s.platform AS site_platform, s.url AS site_url FROM accounts a INNER JOIN sites s ON a.site_id = s.id`
	// Prefix account columns with table alias
	aliasedColumns := `a.id, a.site_id, a.username, a.access_token, a.api_token, a.balance, a.balance_used, a.quota, a.status, a.checkin_enabled, a.last_checkin_at, a.last_balance_refresh, a.extra_config, a.created_at, a.updated_at, a.is_pinned, a.sort_order`
	baseQuery = `SELECT ` + aliasedColumns + `, s.name AS site_name, s.platform AS site_platform, s.url AS site_url FROM accounts a INNER JOIN sites s ON a.site_id = s.id`
	if siteID != nil {
		err := Select(&accounts, baseQuery+` WHERE a.site_id = ? ORDER BY a.sort_order ASC, a.id ASC`, *siteID)
		return accounts, err
	}
	err := Select(&accounts, baseQuery+` ORDER BY a.sort_order ASC, a.id ASC`)
	return accounts, err
}

// ---- AccountToken ----

type AccountToken struct {
	ID          int64          `db:"id" json:"id"`
	AccountID   int64          `db:"account_id" json:"account_id"`
	Name        string         `db:"name" json:"name"`
	Token       string         `db:"token" json:"token"`
	TokenGroup  *string `db:"token_group" json:"token_group"`
	ValueStatus string  `db:"value_status" json:"value_status"`
	Source      *string `db:"source" json:"source"`
	Enabled     *bool   `db:"enabled" json:"enabled"`
	IsDefault   *bool   `db:"is_default" json:"is_default"`
	CreatedAt   *string `db:"created_at" json:"created_at"`
	UpdatedAt   *string `db:"updated_at" json:"updated_at"`
}

func EnsureAccountTokensTable() {
	if driverName == "postgres" {
		DB.MustExec(`CREATE TABLE IF NOT EXISTS account_tokens (
			id SERIAL PRIMARY KEY,
			account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			token TEXT NOT NULL,
			token_group TEXT,
			value_status TEXT DEFAULT 'ready',
			source TEXT,
			enabled BOOLEAN DEFAULT TRUE,
			is_default BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`)
	} else {
		DB.MustExec(`CREATE TABLE IF NOT EXISTS account_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
			account_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			token TEXT NOT NULL,
			token_group TEXT,
			value_status TEXT DEFAULT 'ready',
			source TEXT,
			enabled INTEGER DEFAULT 1,
			is_default INTEGER DEFAULT 0,
			created_at TEXT DEFAULT (datetime('now')),
			updated_at TEXT DEFAULT (datetime('now')),
			FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
		)`)
	}
}

func EnsureSettingsTable() {
	if driverName == "postgres" {
		DB.MustExec(`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT
		)`)
	} else {
		DB.MustExec(`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT
		)`)
	}
}

func ListAccountTokens(accountID int64) ([]AccountToken, error) {
	var tokens []AccountToken
	err := Select(&tokens, `SELECT * FROM account_tokens WHERE account_id = ? ORDER BY id ASC`, accountID)
	return tokens, err
}

func CreateAccountToken(accountID int64, name, token string) (int64, error) {
	now := TimeNow()
	if driverName == "postgres" {
		var id int64
		err := DB.QueryRowx(DB.Rebind(`INSERT INTO account_tokens (account_id, name, token, value_status, source, enabled, is_default, created_at, updated_at) VALUES (?, ?, ?, 'ready', 'manual', true, false, ?, ?) RETURNING id`),
			accountID, name, token, now, now).Scan(&id)
		return id, err
	}
	res, err := Exec(`INSERT INTO account_tokens (account_id, name, token, value_status, source, enabled, is_default, created_at, updated_at) VALUES (?, ?, ?, 'ready', 'manual', 1, 0, ?, ?)`,
		accountID, name, token, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func DeleteAccountToken(id int64) error {
	_, err := Exec(`DELETE FROM account_tokens WHERE id = ?`, id)
	return err
}

// ---- CheckinLog ----

type CheckinLog struct {
	ID        int64          `db:"id" json:"id"`
	AccountID int64          `db:"account_id" json:"account_id"`
	Status    string         `db:"status" json:"status"`
	Message   *string `db:"message" json:"message"`
	Reward    *string `db:"reward" json:"reward"`
	CreatedAt *string `db:"created_at" json:"created_at"`
}

func InsertCheckinLog(accountID int64, status, message, reward string) error {
	now := TimeNow()
	_, err := Exec(`INSERT INTO checkin_logs (account_id, status, message, reward, created_at) VALUES (?, ?, ?, ?, ?)`,
		accountID, status, nilIfEmpty(message), nilIfEmpty(reward), now)
	return err
}

func ListCheckinLogs(accountID *int64, limit, offset int) ([]CheckinLog, int, error) {
	var logs []CheckinLog
	var total int
	if accountID != nil {
		_ = Get(&total, `SELECT COUNT(*) FROM checkin_logs WHERE account_id = ?`, *accountID)
		err := Select(&logs, `SELECT * FROM checkin_logs WHERE account_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`, *accountID, limit, offset)
		return logs, total, err
	}
	_ = Get(&total, `SELECT COUNT(*) FROM checkin_logs`)
	err := Select(&logs, `SELECT * FROM checkin_logs ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	return logs, total, err
}

// CheckinLogWithAccount is a join of checkin_logs + accounts + sites.
type CheckinLogWithAccount struct {
	ID        int64   `db:"id" json:"id"`
	AccountID int64   `db:"account_id" json:"account_id"`
	Status    string  `db:"status" json:"status"`
	Message   *string `db:"message" json:"message"`
	Reward    *string `db:"reward" json:"reward"`
	CreatedAt *string `db:"created_at" json:"created_at"`
	// Joined fields
	AccountUsername *string `db:"account_username" json:"account_username"`
	SiteName        *string `db:"site_name" json:"site_name"`
	SiteURL         *string `db:"site_url" json:"site_url"`
}

func ListCheckinLogsWithAccounts(accountID *int64, limit, offset int) ([]CheckinLogWithAccount, int, error) {
	var logs []CheckinLogWithAccount
	var total int
	baseQuery := `SELECT cl.id, cl.account_id, cl.status, cl.message, cl.reward, cl.created_at,
		a.username AS account_username, s.name AS site_name, s.url AS site_url
		FROM checkin_logs cl
		LEFT JOIN accounts a ON cl.account_id = a.id
		LEFT JOIN sites s ON a.site_id = s.id`
	if accountID != nil {
		_ = Get(&total, `SELECT COUNT(*) FROM checkin_logs WHERE account_id = ?`, *accountID)
		err := Select(&logs, baseQuery+` WHERE cl.account_id = ? ORDER BY cl.created_at DESC LIMIT ? OFFSET ?`, *accountID, limit, offset)
		return logs, total, err
	}
	_ = Get(&total, `SELECT COUNT(*) FROM checkin_logs`)
	err := Select(&logs, baseQuery+` ORDER BY cl.created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	return logs, total, err
}

// ---- Event ----

type Event struct {
	ID          int64          `db:"id" json:"id"`
	Type        string         `db:"type" json:"type"`
	Title       string         `db:"title" json:"title"`
	Message     *string `db:"message" json:"message"`
	Level       string  `db:"level" json:"level"`
	Read        *bool   `db:"read" json:"read"`
	RelatedID   *int64  `db:"related_id" json:"related_id"`
	RelatedType *string `db:"related_type" json:"related_type"`
	CreatedAt   *string `db:"created_at" json:"created_at"`
}

func InsertEvent(typ, title, message, level string, relatedID *int64, relatedType string) error {
	now := TimeNow()
	// SQLite `read`=0, Postgres `read`=false
	readVal := interface{}(0)
	if driverName == "postgres" {
		readVal = false
	}
	_, err := Exec(`INSERT INTO events (type, title, message, level, read, related_id, related_type, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		typ, title, nilIfEmpty(message), level, readVal, relatedID, nilIfEmpty(relatedType), now)
	return err
}

func ListEvents(limit, offset int) ([]Event, int, error) {
	var events []Event
	var total int
	_ = Get(&total, `SELECT COUNT(*) FROM events`)
	err := Select(&events, `SELECT * FROM events ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	return events, total, err
}

func MarkAllEventsRead() error {
	if driverName == "postgres" {
		_, err := Exec(`UPDATE events SET read = true WHERE read = false`)
		return err
	}
	_, err := Exec(`UPDATE events SET read = 1 WHERE read = 0`)
	return err
}

// ---- Setting ----

type Setting struct {
	Key   string         `db:"key" json:"key"`
	Value *string `db:"value" json:"value"`
}

func GetSetting(key string) (*Setting, error) {
	var s Setting
	err := Get(&s, `SELECT * FROM settings WHERE key = ?`, key)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func UpsertSetting(key, value string) error {
	if driverName == "postgres" {
		_, err := Exec(`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, key, value)
		return err
	}
	_, err := Exec(`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?`, key, value, value)
	return err
}

// ---- AccountWithSite (join for checkin/balance) ----

type AccountWithSite struct {
	Account
	SiteName               string  `db:"site_name"`
	SiteURL                string  `db:"site_url"`
	SitePlatform           string  `db:"site_platform"`
	SiteStatus             string  `db:"site_status"`
	SiteProxyURL           *string `db:"site_proxy_url"`
	SiteUseSystemProxy     *bool   `db:"site_use_system_proxy"`
	SiteExternalCheckinURL *string `db:"site_external_checkin_url"`
	SiteCustomHeaders      *string `db:"site_custom_headers"`
}

const accountWithSiteQuery = `
	SELECT a.id, a.site_id, a.username, a.access_token, a.api_token, a.balance, a.balance_used, a.quota,
	       a.status, a.checkin_enabled, a.last_checkin_at, a.last_balance_refresh, a.extra_config,
	       a.created_at, a.updated_at, a.is_pinned, a.sort_order,
	       s.name AS site_name, s.url AS site_url, s.platform AS site_platform, s.status AS site_status, s.proxy_url AS site_proxy_url, s.use_system_proxy AS site_use_system_proxy, s.external_checkin_url AS site_external_checkin_url, s.custom_headers AS site_custom_headers
	FROM accounts a
	INNER JOIN sites s ON a.site_id = s.id
`


func ListCheckinableAccounts() ([]AccountWithSite, error) {
	var rows []AccountWithSite
	if driverName == "postgres" {
		err := Select(&rows, accountWithSiteQuery+`
			WHERE a.checkin_enabled = true AND a.status = 'active'
			ORDER BY a.id ASC
		`)
		return rows, err
	}
	err := Select(&rows, accountWithSiteQuery+`
		WHERE a.checkin_enabled = 1 AND a.status = 'active'
		ORDER BY a.id ASC
	`)
	return rows, err
}

func GetAccountWithSite(accountID int64) (*AccountWithSite, error) {
	var row AccountWithSite
	err := Get(&row, accountWithSiteQuery+` WHERE a.id = ?`, accountID)
	if err != nil {
		return nil, err
	}
	return &row, nil
}
