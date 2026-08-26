package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"config-center/internal/store"
)

// HealthMonitor periodically checks the health of all registered components.
// It runs a background goroutine that executes health checks at a configurable interval.
type HealthMonitor struct {
	store    store.Store
	interval time.Duration
	mu       sync.Mutex
	running  bool
	quit     chan struct{}
	wg       sync.WaitGroup

	// Track last health report for diagnostic access
	lastReport *HealthReport
}

// HealthReport holds the result of a comprehensive health check.
type HealthReport struct {
	Components  []ComponentHealth `json:"components"`
	OverallOK   bool              `json:"overall_ok"`
	Message     string            `json:"message"`
	CheckedAt   time.Time         `json:"checked_at"`
	TotalLatency int64             `json:"total_latency_ms"`
}

// ComponentHealth holds the health status of a single component.
type ComponentHealth struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Latency int64  `json:"latency_ms"`
}

// NewHealthMonitor creates a new HealthMonitor.
func NewHealthMonitor(st store.Store, interval time.Duration) *HealthMonitor {
	return &HealthMonitor{
		store:    st,
		interval: interval,
	}
}

// Start begins the health monitoring loop in a background goroutine.
// If the monitor is already running, Start returns immediately.
func (hm *HealthMonitor) Start() {
	hm.mu.Lock()
	if hm.running {
		hm.mu.Unlock()
		return
	}
	hm.running = true
	hm.quit = make(chan struct{})
	currentQuit := hm.quit
	hm.mu.Unlock()

	hm.wg.Add(1)
	go hm.monitorLoop(currentQuit)
}

// Stop terminates the health monitoring loop.
// It blocks until the monitoring goroutine has exited.
func (hm *HealthMonitor) Stop() {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	if hm.running {
		hm.running = false
		close(hm.quit)
	}
}

// CheckNow performs an immediate health check and returns the report.
func (hm *HealthMonitor) CheckNow() *HealthReport {
	report := &HealthReport{
		Components: make([]ComponentHealth, 0),
		OverallOK:  true,
		CheckedAt:  time.Now(),
	}

	// Check storage
	start := time.Now()
	if err := hm.store.HealthCheck(context.Background()); err != nil {
		report.Components = append(report.Components, ComponentHealth{
			Name:    "storage",
			OK:      false,
			Message: err.Error(),
			Latency: time.Since(start).Milliseconds(),
		})
		report.OverallOK = false
		report.Message = fmt.Sprintf("storage check failed: %v", err)
	} else {
		report.Components = append(report.Components, ComponentHealth{
			Name:    "storage",
			OK:      true,
			Message: "ok",
			Latency: time.Since(start).Milliseconds(),
		})
	}
	report.TotalLatency += time.Since(start).Milliseconds()

	// Update last report
	hm.mu.Lock()
	hm.lastReport = report
	hm.mu.Unlock()

	return report
}

// GetLastReport returns the most recent health report.
func (hm *HealthMonitor) GetLastReport() *HealthReport {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	return hm.lastReport
}

// IsRunning returns whether the monitor is currently active.
func (hm *HealthMonitor) IsRunning() bool {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	return hm.running
}

func (hm *HealthMonitor) monitorLoop(quit <-chan struct{}) {
	defer hm.wg.Done()

	ticker := time.NewTicker(hm.interval)
	defer ticker.Stop()

	// Perform initial check
	report := hm.CheckNow()
	if !report.OverallOK {
		log.Printf("Health check warning: %s", report.Message)
	}

	for {
		select {
		case <-ticker.C:
			report := hm.CheckNow()
			if !report.OverallOK {
				log.Printf("Health check warning: %s", report.Message)
			}
		case <-quit:
			return
		}
	}
}
