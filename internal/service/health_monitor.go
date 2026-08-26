// Package service provides a health monitoring service that periodically checks
// the health of all components and generates status reports.
package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"config-center/internal/store"
	"config-center/pkg/logger"
)

// HealthStatus represents the health status of a component.
type HealthStatus struct {
	// Component is the component name.
	Component string `json:"component"`
	// Status is the health status ("ok", "degraded", "down").
	Status string `json:"status"`
	// Message provides additional status information.
	Message string `json:"message"`
	// Latency is the check latency in milliseconds.
	Latency int64 `json:"latency_ms"`
	// CheckedAt is when the check was performed.
	CheckedAt time.Time `json:"checked_at"`
}

// HealthReport is a comprehensive health report.
type HealthReport struct {
	// Status is the overall system status.
	Status string `json:"status"`
	// Components contains individual component statuses.
	Components []HealthStatus `json:"components"`
	// StartedAt is when the report was generated.
	StartedAt time.Time `json:"started_at"`
	// Duration is the total check duration in milliseconds.
	Duration int64 `json:"duration_ms"`
}

// HealthMonitor periodically checks the health of all system components.
type HealthMonitor struct {
	mu        sync.RWMutex
	store     store.Store
	checkers  []healthChecker
	lastReport *HealthReport
	interval  time.Duration
	logger    *logger.Logger
	quit      chan struct{}
	running   bool
}

// healthChecker is a function that checks a component's health.
type healthChecker struct {
	name    string
	check   func(ctx context.Context) (string, string, int64)
}

// NewHealthMonitor creates a new HealthMonitor.
func NewHealthMonitor(st store.Store, interval time.Duration) *HealthMonitor {
	hm := &HealthMonitor{
		store:    st,
		checkers: make([]healthChecker, 0),
		interval: interval,
		logger:   logger.WithField("monitor", "health"),
		quit:     make(chan struct{}),
	}

	// Register default health checkers
	hm.registerChecker("storage", hm.checkStorage)

	return hm
}

// registerChecker adds a health checker.
func (hm *HealthMonitor) registerChecker(name string, check func(ctx context.Context) (string, string, int64)) {
	hm.checkers = append(hm.checkers, healthChecker{name: name, check: check})
}

// Start begins periodic health monitoring.
func (hm *HealthMonitor) Start() {
	hm.mu.Lock()
	if hm.running {
		hm.mu.Unlock()
		return
	}
	hm.running = true
	hm.mu.Unlock()

	go hm.monitorLoop()
}

// Stop stops periodic health monitoring.
func (hm *HealthMonitor) Stop() {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	if hm.running {
		hm.running = false
		close(hm.quit)
	}
}

// GetReport returns the latest health report.
func (hm *HealthMonitor) GetReport() *HealthReport {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.lastReport
}

// CheckNow performs an immediate health check and returns the report.
func (hm *HealthMonitor) CheckNow(ctx context.Context) *HealthReport {
	start := time.Now()
	var components []HealthStatus
	overallStatus := "ok"

	for _, checker := range hm.checkers {
		checkStart := time.Now()
		status, message, latency := checker.check(ctx)
		checkDuration := time.Since(checkStart)

		components = append(components, HealthStatus{
			Component: checker.name,
			Status:    status,
			Message:   message,
			Latency:   checkDuration.Milliseconds(),
			CheckedAt: time.Now(),
		})

		if status == "down" {
			overallStatus = "down"
		} else if status == "degraded" && overallStatus != "down" {
			overallStatus = "degraded"
		}
		_ = latency
	}

	report := &HealthReport{
		Status:     overallStatus,
		Components: components,
		StartedAt:  time.Now(),
		Duration:   time.Since(start).Milliseconds(),
	}

	hm.mu.Lock()
	hm.lastReport = report
	hm.mu.Unlock()

	return report
}

// monitorLoop is the main monitoring loop.
func (hm *HealthMonitor) monitorLoop() {
	ticker := time.NewTicker(hm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			report := hm.CheckNow(ctx)
			cancel()

			if report.Status != "ok" {
				hm.logger.Warnf("Health report: status=%s, degraded=%d, down=%d",
					report.Status,
					countStatus(report.Components, "degraded"),
					countStatus(report.Components, "down"))
			}
		case <-hm.quit:
			return
		}
	}
}

// checkStorage checks the storage backend health.
func (hm *HealthMonitor) checkStorage(ctx context.Context) (string, string, int64) {
	start := time.Now()
	if err := hm.store.HealthCheck(ctx); err != nil {
		return "down", fmt.Sprintf("storage check failed: %v", err), time.Since(start).Milliseconds()
	}
	return "ok", "storage is healthy", time.Since(start).Milliseconds()
}

// countStatus counts components with a given status.
func countStatus(components []HealthStatus, status string) int {
	count := 0
	for _, c := range components {
		if c.Status == status {
			count++
		}
	}
	return count
}
