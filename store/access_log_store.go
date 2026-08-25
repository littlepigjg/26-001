package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"config-center/config"
)

type AccessLogEntry struct {
	Code      string
	RawURL    string
	Timestamp time.Time
	Status    int
}

type AccessLogStore struct {
	cfg     *config.Config
	mu      sync.Mutex
	entries []AccessLogEntry
	opened  bool
}

func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config must not be nil")
	}
	return &AccessLogStore{
		cfg:     cfg,
		entries: make([]AccessLogEntry, 0),
	}, nil
}

func (s *AccessLogStore) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.opened {
		return fmt.Errorf("access log store already opened")
	}

	s.opened = true
	return nil
}

func (s *AccessLogStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.opened {
		return fmt.Errorf("access log store not opened")
	}

	s.opened = false
	return nil
}

func (s *AccessLogStore) Log(entry AccessLogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.opened {
		return fmt.Errorf("access log store not opened")
	}

	s.entries = append(s.entries, entry)
	return nil
}

func (s *AccessLogStore) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}
