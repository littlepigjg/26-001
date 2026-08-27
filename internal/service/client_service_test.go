package service

import (
	"runtime"
	"testing"
	"time"

	"config-center/internal/store"
)

// TestClientService_RestartCacheCleanup_NoGoroutineLeak reproduces the
// load-test leak at the service layer: NewClientService opens a cache,
// RestartCacheCleanup is called every few tens of milliseconds a handful of
// times, then Close. Before the fix each RestartCacheCleanup left an orphaned
// cleanup goroutine blocked on an abandoned quit channel that Close never
// signaled, so GetActiveGoroutines()/NumGoroutine never returned to zero.
func TestClientService_RestartCacheCleanup_NoGoroutineLeak(t *testing.T) {
	st := store.NewMemoryStore()
	appSvc := NewAppService(st)
	cs := NewClientService(st, appSvc, true)

	// Mirror the load test: rapid restarts a few milliseconds apart.
	for i := 0; i < 5; i++ {
		cs.RestartCacheCleanup(10 * time.Millisecond)
		time.Sleep(5 * time.Millisecond)
	}

	// Exactly one cleanup goroutine plus one cacheMonitor goroutine should be
	// active after the restarts — never one-per-restart.
	if got := cs.GetActiveGoroutines(); got != 2 {
		t.Fatalf("expected 2 active goroutines (cleanup + monitor) after restarts, got %d", got)
	}

	// Warm up and settle a baseline. The cacheMonitor goroutine exits lazily —
	// it only observes the closed flag on its next tick (up to ~100ms later) —
	// so give it time to wind down before asserting.
	cs.Close()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && cs.GetActiveGoroutines() != 0 {
		time.Sleep(20 * time.Millisecond)
	}

	if got := cs.GetActiveGoroutines(); got != 0 {
		t.Fatalf("expected 0 active goroutines after Close, got %d", got)
	}

	// Sanity check against the process-wide goroutine count: Close must not
	// have stranded any cleanup goroutines on abandoned quit channels.
	if got := runtime.NumGoroutine(); got > 20 {
		t.Fatalf("process goroutine count unexpectedly high after close: %d", got)
	}
}

// TestClientService_RestartCacheCleanup_NoGrowthAcrossCycles verifies that
// repeated create/restart/close cycles do not grow the process goroutine count.
func TestClientService_RestartCacheCleanup_NoGrowthAcrossCycles(t *testing.T) {
	st0 := store.NewMemoryStore()
	appSvc0 := NewAppService(st0)
	cs0 := NewClientService(st0, appSvc0, true)
	cs0.Close()
	time.Sleep(20 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	for n := 0; n < 20; n++ {
		st := store.NewMemoryStore()
		appSvc := NewAppService(st)
		cs := NewClientService(st, appSvc, true)
		for i := 0; i < 5; i++ {
			cs.RestartCacheCleanup(1 * time.Millisecond)
		}
		cs.Close()
	}
	// Let any lingering cacheMonitor goroutines observe the closed flag.
	time.Sleep(150 * time.Millisecond)

	if got := runtime.NumGoroutine(); got > baseline+5 {
		t.Fatalf("goroutine leak: baseline=%d, after restart cycles=%d", baseline, got)
	}
}
