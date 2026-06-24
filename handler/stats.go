package handler

import (
	"net/http"
	"time"

	"metapi/aggrsite/db"
)

func GetDashboardStats(w http.ResponseWriter, r *http.Request) {
	// Sites and accounts
	var sitesCount, accountsCount int
	db.DB.Get(&sitesCount, "SELECT COUNT(*) FROM sites")
	db.DB.Get(&accountsCount, "SELECT COUNT(*) FROM accounts")

	// Balances
	var totalBalance, totalUsed float64
	db.DB.Get(&totalBalance, "SELECT COALESCE(SUM(balance), 0) FROM accounts")
	db.DB.Get(&totalUsed, "SELECT COALESCE(SUM(balance_used), 0) FROM accounts")

	// Checkins today
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var checkinsToday, checkinsSuccess int
	db.DB.Get(&checkinsToday, "SELECT COUNT(*) FROM checkin_logs WHERE created_at >= ?", startOfDay)
	db.DB.Get(&checkinsSuccess, "SELECT COUNT(*) FROM checkin_logs WHERE created_at >= ? AND status = 'success'", startOfDay)

	successRate := 0.0
	if checkinsToday > 0 {
		successRate = float64(checkinsSuccess) / float64(checkinsToday) * 100.0
	}

	ok(w, map[string]interface{}{
		"sites":            sitesCount,
		"accounts":         accountsCount,
		"total_balance":    totalBalance,
		"total_used":       totalUsed,
		"checkins_today":   checkinsToday,
		"checkins_success": checkinsSuccess,
		"checkin_rate":     successRate,
	})
}
