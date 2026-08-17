package service

import (
	"testing"
	"time"

	"github.com/robfig/cron/v3"
)

func TestStopSchedulerClearsRunningStateAndJobIDs(t *testing.T) {
	StopScheduler()

	schedulerMu.Lock()
	scheduler = cron.New()
	scheduler.Start()
	checkinJobID = 11
	balanceJobID = 12
	schedulerMu.Unlock()

	StopScheduler()
	status := GetSchedulerStatus()
	if status.Running {
		t.Fatal("scheduler status must report stopped after StopScheduler")
	}
	if checkinJobID != 0 || balanceJobID != 0 {
		t.Fatalf("stale job IDs remain after stop: checkin=%d balance=%d", checkinJobID, balanceJobID)
	}
}

func TestStopManagedRefreshSchedulerCancelsDeferredWake(t *testing.T) {
	StopManagedRefreshScheduler()

	managedRefreshLoopMu.Lock()
	managedRefreshStopCh = make(chan struct{})
	managedRefreshLoopMu.Unlock()
	scheduleManagedRefreshWake(time.Hour)

	managedRefreshLoopMu.Lock()
	hasWake := managedRefreshWakeTimer != nil && !managedRefreshWakeAt.IsZero()
	managedRefreshLoopMu.Unlock()
	if !hasWake {
		t.Fatal("expected a deferred managed-refresh wake")
	}

	StopManagedRefreshScheduler()
	managedRefreshLoopMu.Lock()
	defer managedRefreshLoopMu.Unlock()
	if managedRefreshWakeTimer != nil || !managedRefreshWakeAt.IsZero() {
		t.Fatal("stopping the scheduler must cancel its deferred wake")
	}
}
