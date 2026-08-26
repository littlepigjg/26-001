package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"config-center/internal/config"
)

// AccessRecord is a single access log entry stored by AccessLogStore.
type AccessRecord struct {
	Code      string    `json:"code"`
	Timestamp time.Time `json:"timestamp"`
	RemoteAddr string   `json:"remote_addr"`
	UserAgent  string   `json:"user_agent"`
}

// AccessLogStore is a minimal in-memory store for access log records. It
// mirrors the lifecycle of the URLStore so that the two stores can be used
// together in a typical redirect-pipeline.
type AccessLogStore struct {
	mu    sync.Mutex
	cfg   *config.Config
	logs  []AccessRecord
	index map[string]int
}

// NewAccessLogStore constructs a new AccessLogStore.
func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config must not be nil")
	}
	return &AccessLogStore{
		cfg:   cfg,
		logs:  make([]AccessRecord, 0),
		index: make(map[string]int),
	}, nil
}

// Open prepares the log store for use.
func (a *AccessLogStore) Open(_ context.Context) error {
	return nil
}

// Close releases resources held by the log store.
func (a *AccessLogStore) Close() error {
	return nil
}

// Append records a new access log entry.
func (a *AccessLogStore) Append(rec AccessRecord) error {
	if rec.Code == "" {
		return fmt.Errorf("code is required")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.logs = append(a.logs, rec)
	a.index[rec.Code] = len(a.logs) - 1
	return nil
}

// Len returns the number of log entries currently stored.
func (a *AccessLogStore) Len() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.logs)
}
