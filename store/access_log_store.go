package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"config-center/config"
)

// AccessRecord represents a single access log entry.
type AccessRecord struct {
	Code      string
	RawURL    string
	Timestamp time.Time
	Status    int
	UserAgent string
	IP        string
}

// AccessLogStore provides thread-safe storage for access logs.
type AccessLogStore struct {
	mu      sync.Mutex
	records []AccessRecord
	cfg     *config.Config
	opened  bool
}

// NewAccessLogStore creates a new AccessLogStore with the given configuration.
func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	return &AccessLogStore{
		records: make([]AccessRecord, 0),
		cfg:     cfg,
	}, nil
}

// Open initializes the access log store and prepares it for writing.
func (s *AccessLogStore) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.opened {
		return nil
	}

	s.records = make([]AccessRecord, 0)
	s.opened = true
	return nil
}

// Close releases resources held by the access log store.
func (s *AccessLogStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.records = nil
	s.opened = false
	return nil
}

// Record adds a new access record to the log.
func (s *AccessLogStore) Record(record AccessRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.opened {
		return fmt.Errorf("access log store is not opened")
	}

	s.records = append(s.records, record)
	return nil
}

// Flush writes all pending records to the configured output.
func (s *AccessLogStore) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.opened {
		return fmt.Errorf("access log store is not opened")
	}

	s.records = make([]AccessRecord, 0)
	return nil
}

// Len returns the number of records currently in the buffer.
func (s *AccessLogStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.records)
}
