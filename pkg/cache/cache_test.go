package cache

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestStartCleanup_RestartDoesNotLeakGoroutines reproduces the load-test leak:
// each StartCleanup call used to spawn a fresh goroutine blocked on a brand-new
// quit channel while the previous goroutine kept waiting on the abandoned old
// one. Stop only closed the latest channel, so the orphaned goroutines never
// received the signal and runtime.NumGoroutine() never returned to zero.
func TestStartCleanup_RestartDoesNotLeakGoroutines(t *testing.T) {
	c := NewDefault()

	// Hammer StartCleanup with restarts in quick succession, exactly like
	// RestartCacheCleanup being called every few tens of milliseconds.
	for i := 0; i < 5; i++ {
		c.StartCleanup(10 * time.Millisecond)
		time.Sleep(5 * time.Millisecond)
	}

	// With the fix, at most one cleanup goroutine is ever active.
	if got := c.ActiveGoroutines(); got != 1 {
		t.Fatalf("expected exactly 1 active cleanup goroutine during restarts, got %d", got)
	}

	c.StopAndWait()
	if got := c.ActiveGoroutines(); got != 0 {
		t.Fatalf("expected 0 active goroutines after StopAndWait, got %d", got)
	}
}

// TestStartCleanup_NoGoroutineGrowthAcrossCycles verifies that repeated
// create/restart/stop cycles do not grow the process goroutine count.
func TestStartCleanup_NoGoroutineGrowthAcrossCycles(t *testing.T) {
	// Warm up and take a baseline after one clean stop so the scheduler has
	// settled.
	c0 := NewDefault()
	c0.StartCleanup(1 * time.Millisecond)
	c0.StopAndWait()
	time.Sleep(20 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	for n := 0; n < 20; n++ {
		c := NewDefault()
		for i := 0; i < 5; i++ {
			c.StartCleanup(1 * time.Millisecond)
		}
		c.StopAndWait()
	}
	time.Sleep(50 * time.Millisecond)

	// A handful of goroutines of scheduling noise is fine; a leak would show
	// dozens (one per StartCleanup call that was never reaped).
	if got := runtime.NumGoroutine(); got > baseline+5 {
		t.Fatalf("goroutine leak: baseline=%d, after restart cycles=%d", baseline, got)
	}
}

// TestStartCleanup_ConcurrentNoPanic hammers StartCleanup and Stop from
// multiple goroutines to ensure the channel bookkeeping never double-closes
// a quit channel or closes a nil one.
func TestStartCleanup_ConcurrentNoPanic(t *testing.T) {
	const goroutines = 8
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			c := NewDefault()
			var iw sync.WaitGroup
			iw.Add(4)
			for i := 0; i < 4; i++ {
				go func() {
					defer iw.Done()
					for n := 0; n < 50; n++ {
						c.StartCleanup(time.Millisecond)
					}
				}()
			}
			iw.Wait()
			c.StopAndWait()
		}()
	}
	wg.Wait()
}
