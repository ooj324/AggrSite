package service

import (
	"log/slog"
	"metapi/aggrsite/config"
	"metapi/aggrsite/db"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

var (
	scheduler    *cron.Cron
	schedulerMu  sync.Mutex
	checkinJobID cron.EntryID
	balanceJobID cron.EntryID

	activeCheckinCron string
	activeBalanceCron string
)

type SchedulerStatus struct {
	Running         bool   `json:"running"`
	CheckinCron     string `json:"checkin_cron"`
	NextCheckin     string `json:"next_checkin,omitempty"`
	BalanceCron     string `json:"balance_refresh_cron"`
	NextBalance     string `json:"next_balance_refresh,omitempty"`
}

func getCronSetting(key, fallback string) string {
	s, err := db.GetSetting(key)
	if err == nil && s.Value != nil && *s.Value != "" {
		return *s.Value
	}
	return fallback
}

func StartScheduler() {
	schedulerMu.Lock()
	defer schedulerMu.Unlock()

	if scheduler != nil {
		scheduler.Stop()
	}

	scheduler = cron.New(cron.WithLocation(time.UTC))

	// Determine effective crons
	activeCheckinCron = getCronSetting("CHECKIN_CRON", config.C.CheckinCron)
	activeBalanceCron = getCronSetting("BALANCE_REFRESH_CRON", config.C.BalanceRefreshCron)

	// Schedule checkin
	if activeCheckinCron != "" {
		id, err := scheduler.AddFunc(activeCheckinCron, func() {
			slog.Info("Scheduled checkin triggered")
			results, err := CheckinAll()
			if err != nil {
				slog.Error("Scheduled checkin failed", "err", err)
				return
			}
			successCount := 0
			failCount := 0
			for _, r := range results {
				if r.Result != nil && r.Result.Success {
					successCount++
				} else {
					failCount++
				}
			}
			slog.Info("Scheduled checkin completed", "success", successCount, "fail", failCount)
		})
		if err != nil {
			slog.Error("Failed to schedule checkin cron", "cron", activeCheckinCron, "err", err)
		} else {
			checkinJobID = id
			slog.Info("Checkin cron scheduled", "cron", activeCheckinCron)
		}
	} else {
		checkinJobID = 0
	}

	// Schedule balance refresh
	if activeBalanceCron != "" {
		id, err := scheduler.AddFunc(activeBalanceCron, func() {
			slog.Info("Scheduled balance refresh triggered")
			results, err := RefreshAllBalances()
			if err != nil {
				slog.Error("Scheduled balance refresh failed", "err", err)
				return
			}
			successCount := 0
			for _, r := range results {
				if r.Result != nil && r.Result.Success {
					successCount++
				}
			}
			slog.Info("Scheduled balance refresh completed", "success", successCount, "total", len(results))
		})
		if err != nil {
			slog.Error("Failed to schedule balance cron", "cron", activeBalanceCron, "err", err)
		} else {
			balanceJobID = id
			slog.Info("Balance refresh cron scheduled", "cron", activeBalanceCron)
		}
	} else {
		balanceJobID = 0
	}

	scheduler.Start()
	slog.Info("Scheduler started")
}

func StopScheduler() {
	schedulerMu.Lock()
	defer schedulerMu.Unlock()
	if scheduler != nil {
		scheduler.Stop()
		slog.Info("Scheduler stopped")
	}
}

func ReloadScheduler() {
	slog.Info("Reloading scheduler with new settings...")
	StartScheduler()
}

func GetSchedulerStatus() SchedulerStatus {
	schedulerMu.Lock()
	defer schedulerMu.Unlock()

	status := SchedulerStatus{
		Running:     scheduler != nil,
		CheckinCron: activeCheckinCron,
		BalanceCron: activeBalanceCron,
	}

	if scheduler != nil {
		if checkinJobID > 0 {
			entry := scheduler.Entry(checkinJobID)
			if !entry.Next.IsZero() {
				status.NextCheckin = entry.Next.Format(time.RFC3339)
			}
		}
		if balanceJobID > 0 {
			entry := scheduler.Entry(balanceJobID)
			if !entry.Next.IsZero() {
				status.NextBalance = entry.Next.Format(time.RFC3339)
			}
		}
	}

	return status
}
