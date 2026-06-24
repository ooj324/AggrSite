package db

import (
	"database/sql"
	"time"
)

// ---- helpers ----

func TimeNow() string {
	return time.Now().UTC().Format("2006-01-02 15:04:05")
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
	CreatedAt          sql.NullString `db:"created_at" json:"created_at"`
	UpdatedAt          sql.NullString `db:"updated_at" json:"updated_at"`
	IsPinned           sql.NullBool   `db:"is_pinned" json:"is_pinned"`
	SortOrder          sql.NullInt64  `db:"sort_order" json:"sort_order"`
	ProxyURL           sql.NullString `db:"proxy_url" json:"proxy_url"`
	UseSystemProxy     sql.NullBool   `db:"use_system_proxy" json:"use_system_proxy"`
	CustomHeaders      sql.NullString `db:"custom_headers" json:"custom_headers"`
	ExternalCheckinURL sql.NullString `db:"external_checkin_url" json:"external_checkin_url"`
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
	Name     string `json:"name"`
	URL      string `json:"url"`
	Platform string `json:"platform"`
	Status   string `json:"status"`
}

func CreateSite(in CreateSiteInput) (int64, error) {
	now := TimeNow()
	status := in.Status
	if status == "" {
		status = "active"
	}
	res, err := Exec(`INSERT INTO sites (name, url, platform, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		in.Name, in.URL, in.Platform, status, now, now)
	if err != nil {
		return 0, err
	}
	if driverName == "postgres" {
		// Postgres RETURNING id doesn't work well with res.LastInsertId(), we need QueryRow
		var id int64
		err = DB.QueryRowx(DB.Rebind(`INSERT INTO sites (name, url, platform, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?) RETURNING id`),
			in.Name, in.URL, in.Platform, status, now, now).Scan(&id)
		return id, err
	}
	return res.LastInsertId()
}

func UpdateSite(id int64, in map[string]interface{}) error {
	in["updated_at"] = TimeNow()
	query := "UPDATE sites SET "
	args := []interface{}{}
	i := 0
	for k, v := range in {
		if i > 0 {
			query += ", "
		}
		query += k + " = ?"
		args = append(args, v)
		i++
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

// ---- Account ----

const accountColumns = `id, site_id, username, access_token, api_token, balance, balance_used, quota, status, checkin_enabled, last_checkin_at, last_balance_refresh, extra_config, created_at, updated_at, is_pinned, sort_order`

type Account struct {
	ID                 int64           `db:"id" json:"id"`
	SiteID             int64           `db:"site_id" json:"site_id"`
	Username           sql.NullString  `db:"username" json:"username"`
	AccessToken        string          `db:"access_token" json:"access_token"`
	ApiToken           sql.NullString  `db:"api_token" json:"api_token"`
	Balance            sql.NullFloat64 `db:"balance" json:"balance"`
	BalanceUsed        sql.NullFloat64 `db:"balance_used" json:"balance_used"`
	Quota              sql.NullFloat64 `db:"quota" json:"quota"`
	Status             sql.NullString  `db:"status" json:"status"`
	CheckinEnabled     sql.NullBool    `db:"checkin_enabled" json:"checkin_enabled"`
	LastCheckinAt      sql.NullString  `db:"last_checkin_at" json:"last_checkin_at"`
	LastBalanceRefresh sql.NullString  `db:"last_balance_refresh" json:"last_balance_refresh"`
	ExtraConfig        sql.NullString  `db:"extra_config" json:"extra_config"`
	CreatedAt          sql.NullString  `db:"created_at" json:"created_at"`
	UpdatedAt          sql.NullString  `db:"updated_at" json:"updated_at"`
	IsPinned           sql.NullBool    `db:"is_pinned" json:"is_pinned"`
	SortOrder          sql.NullInt64   `db:"sort_order" json:"sort_order"`
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

type CreateAccountInput struct {
	SiteID         int64  `json:"site_id"`
	Username       string `json:"username"`
	AccessToken    string `json:"access_token"`
	ApiToken       string `json:"api_token"`
	CheckinEnabled bool   `json:"checkin_enabled"`
}

func CreateAccount(in CreateAccountInput) (int64, error) {
	now := TimeNow()
	enabled := 1
	if !in.CheckinEnabled {
		enabled = 0
	}
	if driverName == "postgres" {
		var id int64
		err := DB.QueryRowx(DB.Rebind(`INSERT INTO accounts (site_id, username, access_token, api_token, checkin_enabled, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 'active', ?, ?) RETURNING id`),
			in.SiteID, nilIfEmpty(in.Username), in.AccessToken, nilIfEmpty(in.ApiToken), in.CheckinEnabled, now, now).Scan(&id)
		return id, err
	}
	res, err := Exec(`INSERT INTO accounts (site_id, username, access_token, api_token, checkin_enabled, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 'active', ?, ?)`,
		in.SiteID, nilIfEmpty(in.Username), in.AccessToken, nilIfEmpty(in.ApiToken), enabled, now, now)
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
		if i > 0 {
			query += ", "
		}
		query += k + " = ?"
		args = append(args, v)
		i++
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

// ---- AccountToken ----

type AccountToken struct {
	ID          int64          `db:"id" json:"id"`
	AccountID   int64          `db:"account_id" json:"account_id"`
	Name        string         `db:"name" json:"name"`
	Token       string         `db:"token" json:"token"`
	TokenGroup  sql.NullString `db:"token_group" json:"token_group"`
	ValueStatus string         `db:"value_status" json:"value_status"`
	Source      sql.NullString `db:"source" json:"source"`
	Enabled     sql.NullBool   `db:"enabled" json:"enabled"`
	IsDefault   sql.NullBool   `db:"is_default" json:"is_default"`
	CreatedAt   sql.NullString `db:"created_at" json:"created_at"`
	UpdatedAt   sql.NullString `db:"updated_at" json:"updated_at"`
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
	Message   sql.NullString `db:"message" json:"message"`
	Reward    sql.NullString `db:"reward" json:"reward"`
	CreatedAt sql.NullString `db:"created_at" json:"created_at"`
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

// ---- Event ----

type Event struct {
	ID          int64          `db:"id" json:"id"`
	Type        string         `db:"type" json:"type"`
	Title       string         `db:"title" json:"title"`
	Message     sql.NullString `db:"message" json:"message"`
	Level       string         `db:"level" json:"level"`
	Read        sql.NullBool   `db:"read" json:"read"`
	RelatedID   sql.NullInt64  `db:"related_id" json:"related_id"`
	RelatedType sql.NullString `db:"related_type" json:"related_type"`
	CreatedAt   sql.NullString `db:"created_at" json:"created_at"`
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
	Value sql.NullString `db:"value" json:"value"`
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
	SiteName     string         `db:"site_name"`
	SiteURL      string         `db:"site_url"`
	SitePlatform string         `db:"site_platform"`
	SiteStatus   string         `db:"site_status"`
	SiteProxyURL sql.NullString `db:"site_proxy_url"`
}

const accountWithSiteQuery = `
	SELECT a.id, a.site_id, a.username, a.access_token, a.api_token, a.balance, a.balance_used, a.quota,
	       a.status, a.checkin_enabled, a.last_checkin_at, a.last_balance_refresh, a.extra_config,
	       a.created_at, a.updated_at, a.is_pinned, a.sort_order,
	       s.name AS site_name, s.url AS site_url, s.platform AS site_platform, s.status AS site_status, s.proxy_url AS site_proxy_url
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
