package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"config-center/internal/config"
	"config-center/internal/model"
)

// AccessLogEntry represents a single access log entry.
type AccessLogEntry struct {
	Code      string    `json:"code"`
	RawURL    string    `json:"raw_url"`
	Timestamp time.Time `json:"timestamp"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Status    int       `json:"status"`
}

// AccessLogStore provides persistent storage for access logs.
type AccessLogStore struct {
	mu     sync.RWMutex
	logs   []AccessLogEntry
	path   string
	opened bool
	memStore *MemoryStore
}

// NewAccessLogStore creates a new AccessLogStore with the given configuration.
func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	return &AccessLogStore{
		logs:     make([]AccessLogEntry, 0),
		path:     cfg.Storage.LogPath,
		memStore: NewMemoryStore(),
	}, nil
}

// Open initializes the access log store.
func (s *AccessLogStore) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.opened {
		return fmt.Errorf("access log store already opened")
	}

	if err := s.memStore.HealthCheck(ctx); err != nil {
		return fmt.Errorf("health check failed during open: %w", err)
	}

	s.opened = true
	return nil
}

// Close releases resources held by the access log store.
func (s *AccessLogStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.opened {
		return nil
	}
	s.opened = false
	return s.memStore.Close()
}

// Log records a new access log entry.
func (s *AccessLogStore) Log(entry AccessLogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.opened {
		return fmt.Errorf("access log store is not opened")
	}

	s.logs = append(s.logs, entry)
	return nil
}

// GetLogs returns all access log entries.
func (s *AccessLogStore) GetLogs() []AccessLogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]AccessLogEntry, len(s.logs))
	copy(result, s.logs)
	return result
}

// Clear removes all access log entries.
func (s *AccessLogStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logs = make([]AccessLogEntry, 0)
	return nil
}

// Ensure model import is used
var _ = model.ShortURL{}
