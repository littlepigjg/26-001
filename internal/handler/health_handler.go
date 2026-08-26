package handler

import (
	"fmt"
	"net/http"
	"runtime"
	"time"

	"config-center/pkg/response"
)

// healthResponse represents the health check response.
type healthResponse struct {
	// Status is the health status ("healthy" or "unhealthy").
	Status string `json:"status"`
	// Timestamp is when the health check was performed.
	Timestamp string `json:"timestamp"`
	// Uptime is the server uptime in seconds.
	Uptime float64 `json:"uptime"`
	// GoVersion is the Go runtime version.
	GoVersion string `json:"go_version"`
	// NumGoroutines is the current number of goroutines.
	NumGoroutines int `json:"num_goroutines"`
	// MemoryUsage contains memory statistics.
	MemoryUsage memoryStats `json:"memory_usage"`
}

// memoryStats contains runtime memory statistics.
type memoryStats struct {
	// Alloc is bytes allocated and still in use.
	Alloc uint64 `json:"alloc"`
	// TotalAlloc is total bytes allocated (even if freed).
	TotalAlloc uint64 `json:"total_alloc"`
	// Sys is total bytes of memory obtained from the OS.
	Sys uint64 `json:"sys"`
	// NumGC is the number of completed GC cycles.
	NumGC uint32 `json:"num_gc"`
}

// startTime is when the server started.
var startTime = time.Now()

// Health handles the /health endpoint.
// Returns basic health status and runtime information.
func (h *Handlers) Health(w http.ResponseWriter, _ *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	resp := healthResponse{
		Status:        "healthy",
		Timestamp:     time.Now().Format(time.RFC3339),
		Uptime:        time.Since(startTime).Seconds(),
		GoVersion:     runtime.Version(),
		NumGoroutines: runtime.NumGoroutine(),
		MemoryUsage: memoryStats{
			Alloc:      m.Alloc,
			TotalAlloc: m.TotalAlloc,
			Sys:        m.Sys,
			NumGC:      m.NumGC,
		},
	}

	response.Success(w, resp)
}

// readyResponse represents the readiness check response.
type readyResponse struct {
	// Status is the readiness status ("ready" or "not ready").
	Status string `json:"status"`
	// Checks contains individual readiness check results.
	Checks map[string]string `json:"checks"`
	// Timestamp is when the check was performed.
	Timestamp string `json:"timestamp"`
}

// Ready handles the /ready endpoint.
// Returns readiness status including dependency checks.
func (h *Handlers) Ready(w http.ResponseWriter, _ *http.Request) {
	checks := make(map[string]string)

	// Check storage
	if h.AppService != nil {
		checks["storage"] = "ok"
	} else {
		checks["storage"] = "not_initialized"
	}

	// Check services
	services := map[string]bool{
		"app_service":       h.AppService != nil,
		"config_service":    h.ConfigService != nil,
		"version_service":   h.VersionService != nil,
		"client_service":    h.ClientService != nil,
		"audit_service":     h.AuditService != nil,
		"rollback_service":  h.RollbackService != nil,
		"validation_service": h.ValidationService != nil,
		"diff_service":      h.DiffService != nil,
	}

	allReady := true
	for name, ready := range services {
		if ready {
			checks[name] = "ok"
		} else {
			checks[name] = "not_ready"
			allReady = false
		}
	}

	status := "ready"
	if !allReady {
		status = "not_ready"
	}

	resp := readyResponse{
		Status:    status,
		Checks:    checks,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if allReady {
		response.Success(w, resp)
	} else {
		response.JSON(w, http.StatusServiceUnavailable, resp)
	}
}

// HealthDetailed handles a detailed health check with additional information.
func (h *Handlers) HealthDetailed(w http.ResponseWriter, _ *http.Request) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	response.Success(w, map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"uptime":    time.Since(startTime).String(),
		"runtime": map[string]interface{}{
			"go_version":    runtime.Version(),
			"os":            runtime.GOOS,
			"arch":          runtime.GOARCH,
			"num_cpu":       runtime.NumCPU(),
			"num_goroutine": runtime.NumGoroutine(),
		},
		"memory": map[string]interface{}{
			"alloc":       formatBytes(memStats.Alloc),
			"total_alloc": formatBytes(memStats.TotalAlloc),
			"sys":         formatBytes(memStats.Sys),
			"gc_cycles":   memStats.NumGC,
			"heap_alloc":  formatBytes(memStats.HeapAlloc),
			"heap_sys":    formatBytes(memStats.HeapSys),
			"stack_alloc": formatBytes(memStats.StackInuse),
		},
	})
}

// formatBytes converts bytes to a human-readable format.
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return formatUint(bytes) + " B"
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatUint formats a uint64 as string.
func formatUint(v uint64) string {
	return fmt.Sprintf("%d", v)
}
