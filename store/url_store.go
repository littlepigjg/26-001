package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"config-center/config"
	"config-center/model"
)

type PanicGuardFn func(code, rawURL string) bool

type URLStore struct {
	cfg     *config.Config
	entries map[string]model.ShortURL
	mu      sync.RWMutex
	panicFn PanicGuardFn
	loaded  bool
}

func NewURLStore(cfg *config.Config) (*URLStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	return &URLStore{
		cfg:     cfg,
		entries: make(map[string]model.ShortURL),
	}, nil
}

func (s *URLStore) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loaded {
		return nil
	}

	data, err := os.ReadFile(s.cfg.Storage.GetURLFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			s.loaded = true
			return nil
		}
		return err
	}

	if len(data) > 0 {
		var entries map[string]model.ShortURL
		if err := json.Unmarshal(data, &entries); err != nil {
			return fmt.Errorf("failed to parse url store data: %v", err)
		}
		s.entries = entries
	}

	s.loaded = true
	return nil
}

func (s *URLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.loaded {
		return nil
	}

	data, err := json.Marshal(s.entries)
	if err != nil {
		return fmt.Errorf("failed to marshal url store data: %v", err)
	}

	if err := os.WriteFile(s.cfg.Storage.GetURLFilePath(), data, 0644); err != nil {
		return fmt.Errorf("failed to write url store data: %v", err)
	}

	return nil
}

func (s *URLStore) SetPanicGuard(fn PanicGuardFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.panicFn = fn
}

func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.loaded {
		return fmt.Errorf("store not loaded")
	}

	if !overwrite {
		if _, exists := s.entries[u.Code]; exists {
			return fmt.Errorf("code %s already exists", u.Code)
		}
	}

	if s.panicFn != nil && s.panicFn(u.Code, u.RawURL) {
		s.entries[u.Code] = *u
		return fmt.Errorf("save blocked by panic guard for code: %s", u.Code)
	}

	s.entries[u.Code] = *u

	if s.cfg.Storage.GetFlushOnWrite() {
		if err := s.flushLocked(); err != nil {
			return err
		}
	}

	return nil
}

func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.entries[code]
	if !exists {
		return nil, fmt.Errorf("code %s not found", code)
	}

	result := entry
	return &result, nil
}

func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := make(map[string]model.ShortURL, len(s.entries))
	for k, v := range s.entries {
		snapshot[k] = v
	}
	return snapshot
}

func (s *URLStore) flushLocked() error {
	data, err := json.Marshal(s.entries)
	if err != nil {
		return fmt.Errorf("flush marshal failed: %v", err)
	}

	if err := os.WriteFile(s.cfg.Storage.GetURLFilePath(), data, 0644); err != nil {
		return fmt.Errorf("flush write failed: %v", err)
	}

	return nil
}
