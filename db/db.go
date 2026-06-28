package db

import (
	"database/sql"
	"fmt"
	"log/slog"
	"metapi/aggrsite/config"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

var DB *sqlx.DB
var driverName string
var sqlDriverName string

func Init() {
	dbPath := strings.TrimSpace(config.C.DBUrl)
	var dsn string

	if strings.HasPrefix(dbPath, "postgres://") || strings.HasPrefix(dbPath, "postgresql://") {
		driverName = "postgres"
		sqlDriverName = "pgx"
		sqlx.BindDriver(sqlDriverName, sqlx.DOLLAR)
		cfg, err := pgx.ParseConfig(dbPath)
		if err != nil {
			slog.Error("Failed to parse postgres DSN", "err", err)
			panic(err)
		}
		cfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
		dsn = stdlib.RegisterConnConfig(cfg)
	} else {
		driverName = "sqlite3"
		sqlDriverName = "sqlite3"
		// Ensure directory exists
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			slog.Error("Failed to create data directory", "dir", dir, "err", err)
		}
		dsn = fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on", dbPath)
	}

	var err error
	DB, err = sqlx.Connect(sqlDriverName, dsn)
	if err != nil {
		slog.Error("Failed to connect to database", "path", dbPath, "err", err)
		panic(err)
	}

	if driverName == "sqlite3" {
		DB.SetMaxOpenConns(1) // SQLite best practice
		DB.SetMaxIdleConns(1)
	} else {
		DB.SetMaxOpenConns(25)
		DB.SetMaxIdleConns(5)
		DB.SetConnMaxLifetime(5 * time.Minute)
	}

	slog.Info("Database connected", "driver", driverName, "sql_driver", sqlDriverName)

	EnsureAccountTokensTable()
	EnsureSettingsTable()
	EnsureSiteExternalCheckinColumns()
}

// NowUTC returns a UTC datetime string compatible with the active database
func NowUTC() string {
	if driverName == "postgres" {
		// Just return a timestamp formatted for postgres insertion
		return time.Now().UTC().Format("2006-01-02 15:04:05")
	}
	return "datetime('now')"
}

func IsPostgres() bool {
	return driverName == "postgres"
}

// ---- Query Helpers (auto Rebind) ----

func Exec(query string, args ...interface{}) (sql.Result, error) {
	res, err := DB.Exec(DB.Rebind(query), args...)
	if isPreparedStatementRetryable(err) {
		_ = DB.Ping()
		return DB.Exec(DB.Rebind(query), args...)
	}
	return res, err
}

func Get(dest interface{}, query string, args ...interface{}) error {
	err := DB.Get(dest, DB.Rebind(query), args...)
	if isPreparedStatementRetryable(err) {
		_ = DB.Ping()
		return DB.Get(dest, DB.Rebind(query), args...)
	}
	return err
}

func Select(dest interface{}, query string, args ...interface{}) error {
	err := DB.Select(dest, DB.Rebind(query), args...)
	if isPreparedStatementRetryable(err) {
		_ = DB.Ping()
		return DB.Select(dest, DB.Rebind(query), args...)
	}
	return err
}

func isPreparedStatementRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "unnamed prepared statement does not exist") ||
		strings.Contains(msg, "SQLSTATE 26000") ||
		strings.Contains(msg, "(26000)")
}
