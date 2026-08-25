package store

import (
	"context"
	"fmt"
	"sync"

	"config-center/internal/config"
)

type AccessLogStore struct {
	mu     sync.Mutex
	logs   []string
	cfg    *config.Config
	opened bool
}

func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	return &AccessLogStore{
		logs: make([]string, 0),
		cfg:  cfg,
	}, nil
}

func (s *AccessLogStore) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opened = true
	return nil
}

func (s *AccessLogStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opened = false
	return nil
}

func (s *AccessLogStore) WriteLog(entry string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.opened {
		return fmt.Errorf("access log store not opened")
	}
	s.logs = append(s.logs, entry)
	return nil
}
