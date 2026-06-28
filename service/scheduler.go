package service

import (
	"encoding/json"
	"log/slog"
	"metapi/aggrsite/config"
	"metapi/aggrsite/db"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

var (
	scheduler      *cron.Cron
	schedulerMu    sync.Mutex
	scheduledRunMu sync.Mutex
	checkinJobID   cron.EntryID
	balanceJobID   cron.EntryID

	activeCheckinCron string
	activeBalanceCron string
)

type SchedulerStatus struct {
	Running     bool   `json:"running"`
	CheckinCron string `json:"checkin_cron"`
	NextCheckin string `json:"next_checkin,omitempty"`
	BalanceCron string `json:"balance_refresh_cron"`
	NextBalance string `json:"next_balance_refresh,omitempty"`
	Timezone    string `json:"timezone"`
}

func SettingStringValue(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var parsed string
	if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
		return strings.TrimSpace(parsed)
	}
	return raw
}

func getCronSetting(key, fallback string) string {
	s, err := db.GetSetting(key)
	if err == nil && s.Value != nil && *s.Value != "" {
		return SettingStringValue(*s.Value)
	}
	return fallback
}

func ValidateCronExpr(expr string) error {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil
	}
	_, err := cron.ParseStandard(expr)
	return err
}

func runScheduledTask(name string, fn func() error) {
	if !scheduledRunMu.TryLock() {
		slog.Warn("Scheduled task skipped because another scheduled task is still running", "task", name)
		return
	}
	defer scheduledRunMu.Unlock()
	if err := fn(); err != nil {
		slog.Error("Scheduled task failed", "task", name, "err", err)
	}
}

func StartScheduler() {
	schedulerMu.Lock()
	defer schedulerMu.Unlock()

	if scheduler != nil {
		scheduler.Stop()
	}

	scheduler = cron.New(
		cron.WithLocation(time.Local),
		cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)),
	)

	// Determine effective crons
	activeCheckinCron = getCronSetting("checkin_cron", config.C.CheckinCron)
	activeBalanceCron = getCronSetting("balance_refresh_cron", config.C.BalanceRefreshCron)

	// Schedule checkin
	if activeCheckinCron != "" {
		id, err := scheduler.AddFunc(activeCheckinCron, func() {
			runScheduledTask("checkin", func() error {
				slog.Info("Scheduled checkin triggered")
				results, err := CheckinAll()
				if err != nil {
					return err
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
				return nil
			})
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
			runScheduledTask("balance_refresh", func() error {
				slog.Info("Scheduled balance refresh triggered")
				results, err := RefreshAllBalances()
				if err != nil {
					return err
				}
				successCount := 0
				for _, r := range results {
					if r.Result != nil && r.Result.Success {
						successCount++
					}
				}
				slog.Info("Scheduled balance refresh completed", "success", successCount, "total", len(results))
				return nil
			})
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
		Timezone:    time.Local.String(),
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
