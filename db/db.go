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

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

var DB *sqlx.DB
var driverName string

func Init() {
	dbPath := strings.TrimSpace(config.C.DBUrl)
	var dsn string

	if strings.HasPrefix(dbPath, "postgres://") || strings.HasPrefix(dbPath, "postgresql://") {
		driverName = "postgres"
		dsn = dbPath
	} else {
		driverName = "sqlite3"
		// Ensure directory exists
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			slog.Error("Failed to create data directory", "dir", dir, "err", err)
		}
		dsn = fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on", dbPath)
	}

	var err error
	DB, err = sqlx.Connect(driverName, dsn)
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

	slog.Info("Database connected", "driver", driverName)

	EnsureAccountTokensTable()
	EnsureSettingsTable()
}

// NowUTC returns a UTC datetime string compatible with the active database
func NowUTC() string {
	if driverName == "postgres" {
		// Just return a timestamp formatted for postgres insertion
		return time.Now().UTC().Format("2006-01-02 15:04:05")
	}
	return "datetime('now')"
}

// ---- Query Helpers (auto Rebind) ----

func Exec(query string, args ...interface{}) (sql.Result, error) {
	return DB.Exec(DB.Rebind(query), args...)
}

func Get(dest interface{}, query string, args ...interface{}) error {
	return DB.Get(dest, DB.Rebind(query), args...)
}

func Select(dest interface{}, query string, args ...interface{}) error {
	return DB.Select(dest, DB.Rebind(query), args...)
}
