package config

import (
	"log/slog"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	// Database
	DBUrl   string
	DataDir string

	// Server
	Port       string
	ListenHost string

	// Auth
	AuthToken string

	// Checkin
	CheckinCron string

	// Balance
	BalanceRefreshCron string

	// Notification
	WebhookURL     string
	WebhookEnabled bool
	BarkURL        string
	BarkEnabled    bool

	// Proxy
	SystemProxyURL string
}

var C Config

func Init() {
	// Try loading .env from parent directory (where the original project lives)
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")

	C = Config{
		DataDir:    envStr("DATA_DIR", "../data"),
		Port:       envStr("PORT", "4000"),
		ListenHost: envStr("HOST", "0.0.0.0"),

		AuthToken: envStr("AUTH_TOKEN", "change-me-admin-token"),

		CheckinCron: envStr("CHECKIN_CRON", "0 8 * * *"),

		BalanceRefreshCron: envStr("BALANCE_REFRESH_CRON", "0 * * * *"),

		WebhookURL:     envStr("WEBHOOK_URL", ""),
		WebhookEnabled: envBool("WEBHOOK_ENABLED", true),
		BarkURL:        envStr("BARK_URL", ""),
		BarkEnabled:    envBool("BARK_ENABLED", true),

		SystemProxyURL: envStr("SYSTEM_PROXY_URL", ""),
	}

	// Resolve DB path
	dbURL := strings.TrimSpace(os.Getenv("DB_URL"))
	if dbURL == "" {
		C.DBUrl = C.DataDir + "/hub.db"
	} else {
		C.DBUrl = dbURL
	}

	slog.Info("Config loaded",
		"db", C.DBUrl,
		"port", C.Port,
		"checkin_cron", C.CheckinCron,
		"balance_cron", C.BalanceRefreshCron,
	)
}

func envStr(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func envBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}
