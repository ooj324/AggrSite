package service

import (
	crypto_rand "crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"metapi/aggrsite/config"
	"metapi/aggrsite/db"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

var (
	scheduler    *cron.Cron
	schedulerMu  sync.Mutex
	taskMu       sync.Map // per-task mutex: task name -> *sync.Mutex
	checkinJobID cron.EntryID
	balanceJobID cron.EntryID

	activeCheckinCron string
	activeBalanceCron string

	// Randomized checkin state
	pendingCheckinTimers   []*time.Timer
	pendingCheckinTimersMu sync.Mutex
	pendingCheckinCount    int
)

type SchedulerStatus struct {
	Running                       bool   `json:"running"`
	CheckinCron                   string `json:"checkin_cron"`
	NextCheckin                   string `json:"next_checkin,omitempty"`
	PendingCheckins               int    `json:"pending_checkins"`
	BalanceCron                   string `json:"balance_refresh_cron"`
	NextBalance                   string `json:"next_balance_refresh,omitempty"`
	ManagedRefreshRunning         bool   `json:"managed_refresh_running"`
	ManagedRefreshIntervalSeconds int    `json:"managed_refresh_interval_seconds"`
	ManagedRefreshLeadSeconds     int    `json:"managed_refresh_lead_seconds"`
	Timezone                      string `json:"timezone"`
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

func getIntSetting(key string, fallback int) int {
	s, err := db.GetSetting(key)
	if err == nil && s.Value != nil && *s.Value != "" {
		raw := SettingStringValue(*s.Value)
		if raw != "" {
			var n int
			if _, scanErr := fmt.Sscanf(raw, "%d", &n); scanErr == nil {
				return n
			}
		}
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

func getTaskMu(name string) *sync.Mutex {
	mu, _ := taskMu.LoadOrStore(name, &sync.Mutex{})
	return mu.(*sync.Mutex)
}

func runScheduledTask(name string, fn func() error) {
	mu := getTaskMu(name)
	if !mu.TryLock() {
		slog.Warn("Scheduled task skipped because the same task is still running", "task", name)
		return
	}
	defer mu.Unlock()
	if err := fn(); err != nil {
		slog.Error("Scheduled task failed", "task", name, "err", err)
	}
}

// ---- Cryptographic random helpers ----

// newCryptoRand creates a math/rand.Rand seeded from crypto/rand for
// high-quality, non-predictable random scheduling.
func newCryptoRand() *rand.Rand {
	var seed int64
	if err := binary.Read(crypto_rand.Reader, binary.LittleEndian, &seed); err != nil {
		seed = time.Now().UnixNano()
	}
	return rand.New(rand.NewSource(seed))
}

// randomDuration returns a random Duration in [0, maxMinutes) with
// second-level granularity to avoid clustering on exact minute boundaries.
func randomDuration(rng *rand.Rand, maxMinutes int) time.Duration {
	if maxMinutes <= 0 {
		return 0
	}
	minutes := rng.Intn(maxMinutes)
	seconds := rng.Intn(60)
	return time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
}

// clampCheckinTime adjusts delay so that baseTime+delay falls within the
// configured checkin window [windowStart:00, windowEnd:00).
func clampCheckinTime(baseTime time.Time, delay time.Duration, rng *rand.Rand, windowStart, windowEnd int) time.Duration {
	scheduled := baseTime.Add(delay)
	hour := scheduled.Hour()

	if hour >= windowEnd {
		// Clamp back to (windowEnd-1):00 ~ (windowEnd-1):59
		clampHour := windowEnd - 1
		if clampHour < 0 {
			clampHour = 0
		}
		end := time.Date(scheduled.Year(), scheduled.Month(), scheduled.Day(),
			clampHour, rng.Intn(60), rng.Intn(60), 0, scheduled.Location())
		if end.Before(baseTime) {
			return 0
		}
		return end.Sub(baseTime)
	}
	if hour < windowStart {
		// Push forward to windowStart:00 ~ windowStart:30
		start := time.Date(scheduled.Year(), scheduled.Month(), scheduled.Day(),
			windowStart, rng.Intn(30), rng.Intn(60), 0, scheduled.Location())
		if start.Before(baseTime) {
			return 0
		}
		return start.Sub(baseTime)
	}
	return delay
}

// ---- Randomized checkin scheduling ----

// cancelPendingCheckins cancels all pending randomized checkin timers.
func cancelPendingCheckins() {
	pendingCheckinTimersMu.Lock()
	defer pendingCheckinTimersMu.Unlock()
	for _, t := range pendingCheckinTimers {
		t.Stop()
	}
	pendingCheckinTimers = nil
	pendingCheckinCount = 0
}

func decrementPendingCheckins() {
	pendingCheckinTimersMu.Lock()
	defer pendingCheckinTimersMu.Unlock()
	pendingCheckinCount--
	if pendingCheckinCount < 0 {
		pendingCheckinCount = 0
	}
}

type accountDelay struct {
	AccountID int64
	SiteID    int64
	Username  string
	SiteName  string
	Delay     time.Duration
}

// scheduleRandomCheckins lists all checkinable accounts and schedules each one
// with an independent random delay within the given window. Same-site accounts
// are guaranteed a minimum gap of 2-5 minutes.
func scheduleRandomCheckins(windowMinutes int) {
	cancelPendingCheckins()

	rows, err := db.ListCheckinableAccounts()
	if err != nil {
		slog.Error("Failed to list checkinable accounts", "err", err)
		return
	}
	if len(rows) == 0 {
		slog.Info("No checkinable accounts found")
		return
	}

	rng := newCryptoRand()
	now := time.Now()
	winStart := getIntSetting("checkin_window_start", config.C.CheckinWindowStart)
	winEnd := getIntSetting("checkin_window_end", config.C.CheckinWindowEnd)

	// Generate random delays
	entries := make([]accountDelay, 0, len(rows))
	for _, row := range rows {
		delay := randomDuration(rng, windowMinutes)
		delay = clampCheckinTime(now, delay, rng, winStart, winEnd)
		entries = append(entries, accountDelay{
			AccountID: row.ID,
			SiteID:    row.SiteID,
			Username:  nullStr(row.Username),
			SiteName:  row.SiteName,
			Delay:     delay,
		})
	}

	// Enforce minimum gap between same-site accounts (2-5 min)
	siteGroups := make(map[int64][]int)
	for i, e := range entries {
		siteGroups[e.SiteID] = append(siteGroups[e.SiteID], i)
	}
	for _, indices := range siteGroups {
		if len(indices) <= 1 {
			continue
		}
		sort.Slice(indices, func(a, b int) bool {
			return entries[indices[a]].Delay < entries[indices[b]].Delay
		})
		for k := 1; k < len(indices); k++ {
			minGap := time.Duration(2+rng.Intn(4)) * time.Minute
			prev := entries[indices[k-1]].Delay
			if entries[indices[k]].Delay-prev < minGap {
				entries[indices[k]].Delay = prev + minGap
				entries[indices[k]].Delay = clampCheckinTime(now, entries[indices[k]].Delay, rng, winStart, winEnd)
			}
		}
	}

	// Schedule timers
	pendingCheckinTimersMu.Lock()
	defer pendingCheckinTimersMu.Unlock()

	pendingCheckinTimers = make([]*time.Timer, 0, len(entries))
	pendingCheckinCount = len(entries)

	for _, entry := range entries {
		e := entry
		slog.Info("Scheduling randomized checkin",
			"account_id", e.AccountID,
			"username", e.Username,
			"site", e.SiteName,
			"delay", e.Delay.String(),
			"execute_at", now.Add(e.Delay).Format(time.RFC3339),
		)

		timer := time.AfterFunc(e.Delay, func() {
			defer decrementPendingCheckins()
			runScheduledTask(fmt.Sprintf("checkin_%d", e.AccountID), func() error {
				slog.Info("Randomized checkin executing",
					"account_id", e.AccountID,
					"username", e.Username,
					"site", e.SiteName,
				)
				r, err := CheckinAccount(e.AccountID)
				if err != nil {
					return err
				}
				if r != nil {
					slog.Info("Randomized checkin completed",
						"account_id", e.AccountID,
						"status", r.Status,
						"message", r.Message,
					)
				}
				return nil
			})
		})
		pendingCheckinTimers = append(pendingCheckinTimers, timer)
	}

	slog.Info("All checkins scheduled with random delays",
		"total", len(entries),
		"window_minutes", windowMinutes,
	)
}

func StartScheduler() {
	schedulerMu.Lock()
	defer schedulerMu.Unlock()

	if scheduler != nil {
		scheduler.Stop()
	}
	cancelPendingCheckins()

	scheduler = cron.New(
		cron.WithLocation(time.Local),
		cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)),
	)
	checkinJobID = 0
	balanceJobID = 0

	// Determine effective crons
	activeCheckinCron = getCronSetting("checkin_cron", config.C.CheckinCron)
	activeBalanceCron = getCronSetting("balance_refresh_cron", config.C.BalanceRefreshCron)

	// Schedule checkin (randomized)
	if activeCheckinCron != "" {
		id, err := scheduler.AddFunc(activeCheckinCron, func() {
			// Compute window = time until next cron fire, leave 5 min buffer
			schedulerMu.Lock()
			var windowMinutes int
			if scheduler != nil && checkinJobID > 0 {
				entry := scheduler.Entry(checkinJobID)
				if !entry.Next.IsZero() {
					window := time.Until(entry.Next) - 5*time.Minute
					windowMinutes = int(window.Minutes())
				}
			}
			schedulerMu.Unlock()

			if windowMinutes < 5 {
				windowMinutes = 5 // minimum 5 min window
			}

			slog.Info("Checkin cron triggered, scheduling with random delays",
				"window_minutes", windowMinutes)
			scheduleRandomCheckins(windowMinutes)
		})
		if err != nil {
			slog.Error("Failed to schedule checkin cron", "cron", activeCheckinCron, "err", err)
		} else {
			checkinJobID = id
			slog.Info("Checkin cron scheduled (randomized mode)", "cron", activeCheckinCron)
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
	StartManagedRefreshScheduler()
	slog.Info("Scheduler started")
}

func StopScheduler() {
	schedulerMu.Lock()
	defer schedulerMu.Unlock()
	if scheduler != nil {
		scheduler.Stop()
		scheduler = nil
		slog.Info("Scheduler stopped")
	}
	checkinJobID = 0
	balanceJobID = 0
	cancelPendingCheckins()
	StopManagedRefreshScheduler()
}

func ReloadScheduler() {
	slog.Info("Reloading scheduler with new settings...")
	StartScheduler()
}

func GetSchedulerStatus() SchedulerStatus {
	schedulerMu.Lock()
	defer schedulerMu.Unlock()

	pendingCheckinTimersMu.Lock()
	pending := pendingCheckinCount
	pendingCheckinTimersMu.Unlock()

	managedRunning, managedIntervalSeconds, managedLeadSeconds := GetManagedRefreshSchedulerStatus()
	status := SchedulerStatus{
		Running:                       scheduler != nil,
		CheckinCron:                   activeCheckinCron,
		PendingCheckins:               pending,
		BalanceCron:                   activeBalanceCron,
		ManagedRefreshRunning:         managedRunning,
		ManagedRefreshIntervalSeconds: managedIntervalSeconds,
		ManagedRefreshLeadSeconds:     managedLeadSeconds,
		Timezone:                      time.Local.String(),
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
