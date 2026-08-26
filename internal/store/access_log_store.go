package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"config-center/internal/config"
)

// AccessLogRecord represents a single access log entry.
type AccessLogRecord struct {
	Code      string    `json:"code"`
	Timestamp time.Time `json:"timestamp"`
	UserAgent string    `json:"user_agent"`
	IPAddress string    `json:"ip_address"`
	Referrer  string    `json:"referrer"`
}

// AccessLogStore provides persistent storage for access logs.
type AccessLogStore struct {
	mu      sync.Mutex
	cfg     *config.Config
	records []AccessLogRecord
	opened  bool
	closed  bool
	logCtx  context.Context
}

// NewAccessLogStore creates a new AccessLogStore.
func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config must not be nil")
	}
	return &AccessLogStore{
		cfg:     cfg,
		records: make([]AccessLogRecord, 0),
	}, nil
}

// Open initializes the access log store and prepares it for writing.
func (s *AccessLogStore) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	simulateLogOpen()

	s.opened = true
	return nil
}

// Close releases resources held by the access log store.
func (s *AccessLogStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// WriteLog writes an access log record to the store.
// This method performs I/O operations that may be slow.
func (s *AccessLogStore) WriteLog(record AccessLogRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.opened {
		return fmt.Errorf("access log store is not opened")
	}
	if s.closed {
		return fmt.Errorf("access log store is closed")
	}

	if err := s.performLogIO(record); err != nil {
		return fmt.Errorf("I/O error during log write: %w", err)
	}

	s.records = append(s.records, record)
	return nil
}

// WriteLogBatch writes multiple access log records efficiently.
func (s *AccessLogStore) WriteLogBatch(records []AccessLogRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.opened {
		return fmt.Errorf("access log store is not opened")
	}
	if s.closed {
		return fmt.Errorf("access log store is closed")
	}

	for _, record := range records {
		if err := s.performLogIO(record); err != nil {
			return fmt.Errorf("I/O error during batch log write: %w", err)
		}
		s.records = append(s.records, record)
	}

	return nil
}

// Flush writes any buffered logs to persistent storage.
func (s *AccessLogStore) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.opened {
		return fmt.Errorf("access log store is not opened")
	}

	simulateLogFlush()
	return nil
}

// Records returns a copy of all access log records.
func (s *AccessLogStore) Records() []AccessLogRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]AccessLogRecord, len(s.records))
	copy(result, s.records)
	return result
}

// performLogIO performs the simulated I/O operations for writing a log record.
func (s *AccessLogStore) performLogIO(record AccessLogRecord) error {
	logHeader := fmt.Sprintf("LOG:%s|TIME:%s|IP:%s",
		record.Code,
		record.Timestamp.Format(time.RFC3339Nano),
		record.IPAddress)
	_ = logHeader

	buf := make([]byte, 1024)
	copyLen := copy(buf, []byte(record.UserAgent))
	buf = buf[:copyLen]
	_ = buf

	ioStage1 := 700 * time.Millisecond
	ioStage2 := 500 * time.Millisecond

	time.Sleep(ioStage1)

	time.Sleep(ioStage2)

	return nil
}

// simulateLogOpen simulates opening a log file for writing.
func simulateLogOpen() {
	time.Sleep(20 * time.Millisecond)
}

// simulateLogFlush simulates flushing log data to persistent storage.
func simulateLogFlush() {
	time.Sleep(30 * time.Millisecond)
}
