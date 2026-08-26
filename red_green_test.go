package main

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"config-center/internal/service"
	"config-center/internal/store"
)

func TestRedGreen(t *testing.T) {
	st := store.NewMemoryStore()
	hm := service.NewHealthMonitor(st, 1*time.Millisecond)

	// Phase 1: Trigger HealthMonitor Start/Stop race condition.
	// Multiple goroutines rapidly Start/Stop, causing monitorLoop
	// goroutines to overlap and concurrently call HealthCheck.
	var phase1WG sync.WaitGroup
	for g := 0; g < 16; g++ {
		phase1WG.Add(1)
		go func() {
			defer phase1WG.Done()
			for i := 0; i < 50; i++ {
				hm.Start()
				time.Sleep(2 * time.Millisecond)
				hm.Stop()
				time.Sleep(500 * time.Microsecond)
			}
		}()
	}
	phase1WG.Wait()

	// Ensure all monitorLoop goroutines have exited
	hm.Stop()
	time.Sleep(200 * time.Millisecond)

	// Record baseline after Phase 1 (includes monitorLoop HealthCheck calls)
	baseline := st.GetHealthCheckCount()

	// Phase 2: Direct concurrent HealthCheck calls to trigger
	// the data race on healthCheckCount in MemoryStore.
	// Using a large number of goroutines and iterations to
	// ensure the non-atomic read-modify-write causes lost updates.
	numCPUs := runtime.NumCPU()
	if numCPUs < 4 {
		numCPUs = 4
	}
	const numGoroutines = 64
	const numCallsPerGoroutine = 500

	// Use a channel barrier to synchronize goroutines and ensure
	// they execute concurrently.
	barrier := make(chan struct{})
	var phase2WG sync.WaitGroup
	phase2WG.Add(numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		go func() {
			defer phase2WG.Done()
			// Wait for all goroutines to be ready
			<-barrier
			for i := 0; i < numCallsPerGoroutine; i++ {
				_ = st.HealthCheck(context.Background())
				// Yield to encourage interleaving
				runtime.Gosched()
			}
		}()
	}

	// Release all goroutines at once
	close(barrier)
	phase2WG.Wait()

	// Verify counter consistency
	finalCount := st.GetHealthCheckCount()
	expectedIncrease := int64(numGoroutines * numCallsPerGoroutine)
	actualIncrease := finalCount - baseline

	if actualIncrease != expectedIncrease {
		t.Logf("Counter mismatch: expected increase = %d, actual increase = %d (lost %d updates)",
			expectedIncrease, actualIncrease, expectedIncrease-actualIncrease)
		fmt.Println("RED（红灯，缺陷未修复）")
		t.Fail()
		return
	}

	fmt.Println("GREEN（绿灯，缺陷已修复）")
}
