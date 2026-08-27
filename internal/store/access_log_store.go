package store

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"config-center/internal/config"
)

type AccessLogEntry struct {
	Code      string
	RawURL    string
	Timestamp time.Time
	Status    int
}

type AccessLogStore struct {
	mu     sync.Mutex
	cfg    *config.Config
	file   *os.File
	closed bool
	ready  bool
}

func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	return &AccessLogStore{
		cfg: cfg,
	}, nil
}

func (s *AccessLogStore) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("access log store is closed")
	}
	if s.ready {
		return fmt.Errorf("access log store already open")
	}

	path := s.cfg.Storage.GetLogFilePath()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	s.file = f
	s.ready = true
	return nil
}

func (s *AccessLogStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true
	s.ready = false
	if s.file != nil {
		s.file.Close()
	}
	return nil
}

func (s *AccessLogStore) Log(entry AccessLogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.ready {
		return fmt.Errorf("access log store not ready")
	}
	if s.closed {
		return fmt.Errorf("access log store is closed")
	}

	line := fmt.Sprintf("[%s] %s -> %d %s\n",
		entry.Timestamp.Format(time.RFC3339),
		entry.Code,
		entry.Status,
		entry.RawURL,
	)
	_, err := s.file.WriteString(line)
	return err
}

func (s *AccessLogStore) LogWithContext(ctx context.Context, entry AccessLogEntry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.Log(entry)
}

func (s *AccessLogStore) Ready() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ready && !s.closed
}
