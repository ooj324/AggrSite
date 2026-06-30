package db

import (
	"encoding/json"
	"strings"
	"time"
)

// ---- helpers ----

func TimeNow() string {
	return time.Now().Local().Format("2006-01-02T15:04:05")
}

func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func boolDBValue(v bool) interface{} {
	if driverName == "postgres" {
		return v
	}
	if v {
		return 1
	}
	return 0
}

// ---- Site ----

const siteColumns = `id, name, url, platform, status, created_at, updated_at, is_pinned, sort_order, proxy_url, use_system_proxy, custom_headers, external_checkin_url, external_checkin_method, external_checkin_auth_header, external_checkin_auth_prefix, external_checkin_body`

type Site struct {
	ID                        int64   `db:"id" json:"id"`
	Name                      string  `db:"name" json:"name"`
	URL                       string  `db:"url" json:"url"`
	Platform                  string  `db:"platform" json:"platform"`
	Status                    string  `db:"status" json:"status"`
	CreatedAt                 *string `db:"created_at" json:"created_at"`
	UpdatedAt                 *string `db:"updated_at" json:"updated_at"`
	IsPinned                  *bool   `db:"is_pinned" json:"is_pinned"`
	SortOrder                 *int64  `db:"sort_order" json:"sort_order"`
	ProxyURL                  *string `db:"proxy_url" json:"proxy_url"`
	UseSystemProxy            *bool   `db:"use_system_proxy" json:"use_system_proxy"`
	CustomHeaders             *string `db:"custom_headers" json:"custom_headers"`
	ExternalCheckinURL        *string `db:"external_checkin_url" json:"external_checkin_url"`
	ExternalCheckinMethod     *string `db:"external_checkin_method" json:"external_checkin_method"`
	ExternalCheckinAuthHeader *string `db:"external_checkin_auth_header" json:"external_checkin_auth_header"`
	ExternalCheckinAuthPrefix *string `db:"external_checkin_auth_prefix" json:"external_checkin_auth_prefix"`
	ExternalCheckinBody       *string `db:"external_checkin_body" json:"external_checkin_body"`
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
	Name                      string  `json:"name"`
	URL                       string  `json:"url"`
	Platform                  string  `json:"platform"`
	Status                    string  `json:"status"`
	ProxyURL                  *string `json:"proxy_url"`
	UseSystemProxy            *bool   `json:"use_system_proxy"`
	ExternalCheckinURL        *string `json:"external_checkin_url"`
	ExternalCheckinMethod     *string `json:"external_checkin_method"`
	ExternalCheckinAuthHeader *string `json:"external_checkin_auth_header"`
	ExternalCheckinAuthPrefix *string `json:"external_checkin_auth_prefix"`
	ExternalCheckinBody       *string `json:"external_checkin_body"`
	CustomHeaders             *string `json:"custom_headers"`
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

	customHeaders := in.CustomHeaders
	if driverName == "postgres" && customHeaders != nil && *customHeaders == "" {
		emptyJson := "{}"
		customHeaders = &emptyJson
	}

	query := `INSERT INTO sites (name, url, platform, status, proxy_url, use_system_proxy, external_checkin_url, external_checkin_method, external_checkin_auth_header, external_checkin_auth_prefix, external_checkin_body, custom_headers, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	if driverName == "postgres" {
		var id int64
		err := DB.QueryRowx(DB.Rebind(query+` RETURNING id`),
			in.Name, in.URL, in.Platform, status, in.ProxyURL, useSystemProxyVal, in.ExternalCheckinURL, in.ExternalCheckinMethod, in.ExternalCheckinAuthHeader, in.ExternalCheckinAuthPrefix, in.ExternalCheckinBody, customHeaders, now, now).Scan(&id)
		return id, err
	}

	res, err := Exec(query, in.Name, in.URL, in.Platform, status, in.ProxyURL, useSystemProxyVal, in.ExternalCheckinURL, in.ExternalCheckinMethod, in.ExternalCheckinAuthHeader, in.ExternalCheckinAuthPrefix, in.ExternalCheckinBody, customHeaders, now, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func UpdateSite(id int64, in map[string]interface{}) error {
	in["updated_at"] = TimeNow()

	// Handle custom_headers for Postgres jsonb compatibility
	if ch, ok := in["custom_headers"].(string); ok && ch == "" {
		// Replace empty string with valid JSON empty object or nil
		if driverName == "postgres" {
			in["custom_headers"] = "{}"
		} else {
			in["custom_headers"] = nil
		}
	}

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

const accountColumns = `id, site_id, username, access_token, api_token, balance, balance_used, quota, unit_cost, value_score, status, checkin_enabled, last_checkin_at, last_balance_refresh, extra_config, created_at, updated_at, is_pinned, sort_order`

type Account struct {
	ID                 int64    `db:"id" json:"id"`
	SiteID             int64    `db:"site_id" json:"site_id"`
	Username           *string  `db:"username" json:"username"`
	AccessToken        string   `db:"access_token" json:"access_token"`
	ApiToken           *string  `db:"api_token" json:"api_token"`
	Balance            *float64 `db:"balance" json:"balance"`
	BalanceUsed        *float64 `db:"balance_used" json:"balance_used"`
	Quota              *float64 `db:"quota" json:"quota"`
	UnitCost           *float64 `db:"unit_cost" json:"unit_cost"`
	ValueScore         *float64 `db:"value_score" json:"value_score"`
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
	SiteID         int64  `json:"site_id"`
	Username       string `json:"username"`
	AccessToken    string `json:"access_token"`
	ApiToken       string `json:"api_token"`
	CheckinEnabled bool   `json:"checkin_enabled"`
	PlatformUserID *int64 `json:"platformUserId"`
	CredentialMode string `json:"credentialMode"`
	SkipModelFetch bool   `json:"skipModelFetch"`
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
	aliasedColumns := `a.id, a.site_id, a.username, a.access_token, a.api_token, a.balance, a.balance_used, a.quota, a.unit_cost, a.value_score, a.status, a.checkin_enabled, a.last_checkin_at, a.last_balance_refresh, a.extra_config, a.created_at, a.updated_at, a.is_pinned, a.sort_order`
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
	ID          int64   `db:"id" json:"id"`
	AccountID   int64   `db:"account_id" json:"account_id"`
	Name        string  `db:"name" json:"name"`
	Token       string  `db:"token" json:"token"`
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
			id SERIAL PRIMARY KEY,
			key TEXT UNIQUE NOT NULL,
			value TEXT NOT NULL
		)`)
	} else {
		DB.MustExec(`CREATE TABLE IF NOT EXISTS settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
			key TEXT UNIQUE NOT NULL,
			value TEXT NOT NULL
		)`)
	}
}

// EnsureSiteExternalCheckinColumns automatically adds extended configuration fields
// if they don't exist in the sites table. This allows AggrSite to be fully backward-compatible
// and run independent of the main app's migrations.
func EnsureSiteExternalCheckinColumns() {
	if driverName == "postgres" {
		DB.Exec(`ALTER TABLE sites ALTER COLUMN external_checkin_url TYPE TEXT`)
	}
	// ensure the new columns exist (we ignore errors because they might already exist)
	DB.Exec(`ALTER TABLE sites ADD COLUMN external_checkin_method TEXT`)
	DB.Exec(`ALTER TABLE sites ADD COLUMN external_checkin_auth_header TEXT`)
	DB.Exec(`ALTER TABLE sites ADD COLUMN external_checkin_auth_prefix TEXT`)
	DB.Exec(`ALTER TABLE sites ADD COLUMN external_checkin_body TEXT`)
}

func ListAccountTokens(accountID int64) ([]AccountToken, error) {
	var tokens []AccountToken
	err := Select(&tokens, `SELECT * FROM account_tokens WHERE account_id = ? ORDER BY id ASC`, accountID)
	return tokens, err
}

func CreateAccountToken(accountID int64, name, token string) (int64, error) {
	now := TimeNow()
	name = strings.TrimSpace(name)
	token = strings.TrimSpace(token)
	if name == "" {
		name = "default"
	}
	if token == "" {
		return 0, nil
	}

	var existing AccountToken
	if err := Get(&existing, `SELECT * FROM account_tokens WHERE account_id = ? AND token = ? LIMIT 1`, accountID, token); err == nil {
		_ = UpdateAccountToken(existing.ID, map[string]interface{}{
			"name":         name,
			"value_status": "ready",
			"enabled":      boolDBValue(true),
			"updated_at":   now,
		})
		return existing.ID, nil
	}

	var count int
	_ = Get(&count, `SELECT COUNT(*) FROM account_tokens WHERE account_id = ?`, accountID)
	isDefault := count == 0
	defaultVal := boolDBValue(isDefault)
	enabledVal := boolDBValue(true)

	if driverName == "postgres" {
		var id int64
		err := DB.QueryRowx(DB.Rebind(`INSERT INTO account_tokens (account_id, name, token, value_status, source, enabled, is_default, created_at, updated_at) VALUES (?, ?, ?, 'ready', 'manual', ?, ?, ?, ?) RETURNING id`),
			accountID, name, token, enabledVal, defaultVal, now, now).Scan(&id)
		if err == nil && isDefault {
			_ = UpdateAccount(accountID, map[string]interface{}{"api_token": token})
		}
		return id, err
	}
	res, err := Exec(`INSERT INTO account_tokens (account_id, name, token, value_status, source, enabled, is_default, created_at, updated_at) VALUES (?, ?, ?, 'ready', 'manual', ?, ?, ?, ?)`,
		accountID, name, token, enabledVal, defaultVal, now, now)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err == nil && isDefault {
		_ = UpdateAccount(accountID, map[string]interface{}{"api_token": token})
	}
	return id, err
}

func UpdateAccountToken(id int64, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	allowed := map[string]bool{
		"name":         true,
		"token":        true,
		"token_group":  true,
		"value_status": true,
		"source":       true,
		"enabled":      true,
		"is_default":   true,
		"updated_at":   true,
	}
	query := "UPDATE account_tokens SET "
	args := []interface{}{}
	i := 0
	for k, v := range fields {
		if !allowed[k] {
			continue
		}
		if i > 0 {
			query += ", "
		}
		query += k + " = ?"
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

func DeleteAccountToken(id int64) error {
	_, err := Exec(`DELETE FROM account_tokens WHERE id = ?`, id)
	return err
}

// ---- CheckinLog ----

type CheckinLog struct {
	ID        int64   `db:"id" json:"id"`
	AccountID int64   `db:"account_id" json:"account_id"`
	Status    string  `db:"status" json:"status"`
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

type CheckinLogFilter struct {
	AccountID *int64
	Status    string
	Start     string
	End       string
}

func ListCheckinLogsWithAccounts(filter CheckinLogFilter, limit, offset int) ([]CheckinLogWithAccount, int, error) {
	var logs []CheckinLogWithAccount
	var total int
	baseQuery := `SELECT cl.id, cl.account_id, cl.status, cl.message, cl.reward, cl.created_at,
		a.username AS account_username, s.name AS site_name, s.url AS site_url
		FROM checkin_logs cl
		LEFT JOIN accounts a ON cl.account_id = a.id
		LEFT JOIN sites s ON a.site_id = s.id`
	conditions := []string{}
	args := []interface{}{}
	if filter.AccountID != nil {
		conditions = append(conditions, "cl.account_id = ?")
		args = append(args, *filter.AccountID)
	}
	if filter.Status != "" && filter.Status != "all" {
		conditions = append(conditions, "cl.status = ?")
		args = append(args, filter.Status)
	}
	if filter.Start != "" {
		conditions = append(conditions, "cl.created_at >= ?")
		args = append(args, filter.Start)
	}
	if filter.End != "" {
		conditions = append(conditions, "cl.created_at < ?")
		args = append(args, filter.End)
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	countArgs := append([]interface{}{}, args...)
	_ = Get(&total, `SELECT COUNT(*) FROM checkin_logs cl`+where, countArgs...)
	args = append(args, limit, offset)
	err := Select(&logs, baseQuery+where+` ORDER BY cl.created_at DESC LIMIT ? OFFSET ?`, args...)
	return logs, total, err
}

// ---- Event ----

type Event struct {
	ID          int64   `db:"id" json:"id"`
	Type        string  `db:"type" json:"type"`
	Title       string  `db:"title" json:"title"`
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

type EventFilter struct {
	Level string
	Type  string
	Start string
	End   string
}

func ListEvents(filter EventFilter, limit, offset int) ([]Event, int, error) {
	var events []Event
	var total int
	conditions := []string{}
	args := []interface{}{}
	if filter.Level != "" && filter.Level != "all" {
		conditions = append(conditions, "COALESCE(level, 'info') = ?")
		args = append(args, filter.Level)
	}
	if filter.Type != "" && filter.Type != "all" {
		conditions = append(conditions, "COALESCE(type, 'system') = ?")
		args = append(args, filter.Type)
	}
	if filter.Start != "" {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, filter.Start)
	}
	if filter.End != "" {
		conditions = append(conditions, "created_at < ?")
		args = append(args, filter.End)
	}
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	countArgs := append([]interface{}{}, args...)
	_ = Get(&total, `SELECT COUNT(*) FROM events`+where, countArgs...)
	args = append(args, limit, offset)
	err := Select(&events, `SELECT id, COALESCE(type, 'system') AS type, COALESCE(title, 'Event') AS title, message, COALESCE(level, 'info') AS level, read, related_id, related_type, created_at FROM events`+where+` ORDER BY created_at DESC LIMIT ? OFFSET ?`, args...)
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
	Key   string  `db:"key" json:"key"`
	Value *string `db:"value" json:"value"`
}

func NormalizeSettingKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

func GetSetting(key string) (*Setting, error) {
	var s Setting
	normalizedKey := NormalizeSettingKey(key)
	err := Get(&s, `SELECT * FROM settings WHERE LOWER(key) = LOWER(?) ORDER BY CASE WHEN key = ? THEN 0 ELSE 1 END LIMIT 1`, normalizedKey, normalizedKey)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func UpsertSetting(key, value string) error {
	key = NormalizeSettingKey(key)
	if existing, err := GetSetting(key); err == nil && existing.Key != key {
		_, err := Exec(`UPDATE settings SET key = ?, value = ? WHERE key = ?`, key, value, existing.Key)
		return err
	}
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
	SiteName                      string  `db:"site_name"`
	SiteURL                       string  `db:"site_url"`
	SitePlatform                  string  `db:"site_platform"`
	SiteStatus                    string  `db:"site_status"`
	SiteSortOrder                 *int64  `db:"site_sort_order"`
	SiteProxyURL                  *string `db:"site_proxy_url"`
	SiteUseSystemProxy            *bool   `db:"site_use_system_proxy"`
	SiteCustomHeaders             *string `db:"site_custom_headers"`
	SiteExternalCheckinURL        *string `db:"site_external_checkin_url"`
	SiteExternalCheckinMethod     *string `db:"site_external_checkin_method"`
	SiteExternalCheckinAuthHeader *string `db:"site_external_checkin_auth_header"`
	SiteExternalCheckinAuthPrefix *string `db:"site_external_checkin_auth_prefix"`
	SiteExternalCheckinBody       *string `db:"site_external_checkin_body"`
}

const accountWithSiteQuery = `
	SELECT a.id, a.site_id, a.username, a.access_token, a.api_token, a.balance, a.balance_used, a.quota, a.unit_cost, a.value_score,
	       a.status, a.checkin_enabled, a.last_checkin_at, a.last_balance_refresh, a.extra_config,
	       a.created_at, a.updated_at, a.is_pinned, a.sort_order,
	       s.name AS site_name, s.url AS site_url, s.platform AS site_platform, s.status AS site_status, s.sort_order AS site_sort_order,
	       s.proxy_url AS site_proxy_url, s.use_system_proxy AS site_use_system_proxy, s.custom_headers AS site_custom_headers,
	       s.external_checkin_url AS site_external_checkin_url, s.external_checkin_method AS site_external_checkin_method,
	       s.external_checkin_auth_header AS site_external_checkin_auth_header, s.external_checkin_auth_prefix AS site_external_checkin_auth_prefix,
	       s.external_checkin_body AS site_external_checkin_body
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

func ListActiveAccountsWithSiteByPlatform(platform string) ([]AccountWithSite, error) {
	var rows []AccountWithSite
	err := Select(&rows, accountWithSiteQuery+`
		WHERE COALESCE(NULLIF(LOWER(TRIM(a.status)), ''), 'active') = 'active'
		  AND COALESCE(NULLIF(LOWER(TRIM(s.status)), ''), 'active') = 'active'
		  AND COALESCE(LOWER(TRIM(s.platform)), '') = ?
		ORDER BY a.id ASC
	`, strings.ToLower(strings.TrimSpace(platform)))
	return rows, err
}
