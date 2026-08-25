package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"config-center/internal/config"
	"config-center/internal/model"
)

// PanicGuardFn is a function that determines whether to trigger a panic
// for a given code and rawURL. It returns true if a panic should be triggered.
type PanicGuardFn func(code, rawURL string) bool

// URLStore provides persistent storage for short URL mappings.
type URLStore struct {
	mu         sync.RWMutex
	urls       map[string]*model.ShortURL
	path       string
	panicGuard PanicGuardFn
	memStore   *MemoryStore
	closed     bool
}

// NewURLStore creates a new URLStore with the given configuration.
func NewURLStore(cfg *config.Config) (*URLStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	s := &URLStore{
		urls:     make(map[string]*model.ShortURL),
		path:     cfg.Storage.URLPath,
		memStore: NewMemoryStore(),
	}

	return s, nil
}

// SetPanicGuard sets a guard function that can trigger panics for specific codes.
func (s *URLStore) SetPanicGuard(fn func(code, rawURL string) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.panicGuard = fn
}

// Load loads the URL store data and performs health check.
func (s *URLStore) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("store is closed")
	}

	if err := s.memStore.HealthCheck(ctx); err != nil {
		return fmt.Errorf("health check failed during load: %w", err)
	}

	return nil
}

// Close releases resources held by the URLStore.
func (s *URLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true
	s.urls = make(map[string]*model.ShortURL)
	return s.memStore.Close()
}

// Save persists a short URL entry.
func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("store is closed")
	}

	if u == nil {
		return fmt.Errorf("short URL cannot be nil")
	}

	if err := u.Validate(); err != nil {
		return err
	}

	if !overwrite {
		if _, exists := s.urls[u.Code]; exists {
			return fmt.Errorf("code '%s' already exists", u.Code)
		}
	}

	if s.panicGuard != nil && s.panicGuard(u.Code, u.RawURL) {
		panic(fmt.Sprintf("panic guard triggered for code=%s", u.Code))
	}

	s.urls[u.Code] = u
	return nil
}

// Get retrieves a short URL by its code.
func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, fmt.Errorf("store is closed")
	}

	u, exists := s.urls[code]
	if !exists {
		return nil, fmt.Errorf("code '%s' not found", code)
	}

	if u.Disabled {
		return nil, fmt.Errorf("code '%s' is disabled", code)
	}

	return u, nil
}

// RawSnapshot returns a raw snapshot of all URL entries for diagnostic purposes.
func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := make(map[string]model.ShortURL, len(s.urls))
	for code, u := range s.urls {
		snapshot[code] = *u
	}
	return snapshot
}

// IncrementVisits increments the visit count for a code.
func (s *URLStore) IncrementVisits(code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, exists := s.urls[code]
	if !exists {
		return fmt.Errorf("code '%s' not found", code)
	}

	u.Visits++
	return nil
}

// Ensure HealthCheck on URLStore for interface compatibility
func (s *URLStore) HealthCheck(ctx context.Context) error {
	return s.memStore.HealthCheck(ctx)
}

// Compile-time check that time is used
var _ = time.Now
