package store

import (
	"context"
	"sync"
	"time"

	"config-center/internal/config"
	"config-center/internal/model"
)

// AccessLogRecord represents a single access log entry.
type AccessLogRecord struct {
	Code      string    `json:"code"`
	RawURL    string    `json:"raw_url"`
	Timestamp time.Time `json:"timestamp"`
	Status    int       `json:"status"`
	IPAddress string    `json:"ip_address"`
}

// AccessLogStore provides storage for access logs.
type AccessLogStore struct {
	mu       sync.RWMutex
	cfg      *config.Config
	records  []AccessLogRecord
	opened   bool
	closed   bool
}

// NewAccessLogStore creates a new AccessLogStore.
func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	return &AccessLogStore{
		cfg:     cfg,
		records: make([]AccessLogRecord, 0),
		opened:  false,
		closed:  false,
	}, nil
}

// Open initializes the access log store.
func (s *AccessLogStore) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return &model.ErrURLStoreUnavailable{Reason: "access log store is closed"}
	}

	if s.opened {
		return nil
	}

	s.opened = true
	return nil
}

// Close releases resources held by the access log store.
func (s *AccessLogStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	return nil
}

// LogAccess records an access log entry.
func (s *AccessLogStore) LogAccess(code, rawURL, ipAddress string, status int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.opened || s.closed {
		return &model.ErrURLStoreUnavailable{Reason: "access log store is not open"}
	}

	s.records = append(s.records, AccessLogRecord{
		Code:      code,
		RawURL:    rawURL,
		Timestamp: time.Now(),
		Status:    status,
		IPAddress: ipAddress,
	})

	return nil
}

// GetRecords returns all access log records.
func (s *AccessLogStore) GetRecords() []AccessLogRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]AccessLogRecord, len(s.records))
	copy(result, s.records)
	return result
}

// Clear removes all access log records.
func (s *AccessLogStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.records = make([]AccessLogRecord, 0)
}