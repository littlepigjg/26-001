package store

import (
	"context"
	"fmt"
	"os"
	"sync"

	"config-center/config"
)

type AccessLogEntry struct {
	Code      string `json:"code"`
	RawURL    string `json:"raw_url"`
	Timestamp string `json:"timestamp"`
}

type AccessLogStore struct {
	cfg     *config.Config
	mu      sync.Mutex
	entries []AccessLogEntry
	open    bool
}

func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	return &AccessLogStore{
		cfg:     cfg,
		entries: make([]AccessLogEntry, 0),
	}, nil
}

func (s *AccessLogStore) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.open {
		return nil
	}

	f, err := os.OpenFile(s.cfg.Storage.GetLogFilePath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open access log: %v", err)
	}
	f.Close()

	s.open = true
	return nil
}

func (s *AccessLogStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.open {
		return nil
	}

	s.open = false
	return nil
}

func (s *AccessLogStore) IsOpen() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.open
}

func (s *AccessLogStore) Append(entry AccessLogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.open {
		return fmt.Errorf("access log store not open")
	}

	f, err := os.OpenFile(s.cfg.Storage.GetLogFilePath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to append access log: %v", err)
	}
	defer f.Close()

	line := fmt.Sprintf("%s\t%s\t%s\n", entry.Timestamp, entry.Code, entry.RawURL)
	if _, err := f.WriteString(line); err != nil {
		return fmt.Errorf("failed to write access log: %v", err)
	}

	s.entries = append(s.entries, entry)
	return nil
}
