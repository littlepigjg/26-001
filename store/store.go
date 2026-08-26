// Package store provides the storage layer for the URL shortener service.
package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"config-center/config"
	"config-center/model"
)

// PanicGuardFn is a function that can be set to guard against certain panic conditions.
// It returns true if the code/rawURL combination should trigger a guarded recovery.
type PanicGuardFn func(code, rawURL string) bool

// URLStore stores and manages short URL data.
type URLStore struct {
	cfg        *config.Config
	mu         sync.RWMutex
	urls       map[string]model.ShortURL
	panicGuard PanicGuardFn
	closed     bool
}

// NewURLStore creates a new URLStore with the given configuration.
func NewURLStore(cfg *config.Config) (*URLStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	return &URLStore{
		cfg:  cfg,
		urls: make(map[string]model.ShortURL),
	}, nil
}

// Load loads URL data from persistent storage.
func (s *URLStore) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("store is closed")
	}
	return nil
}

// Close releases resources held by the URLStore.
func (s *URLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// SetPanicGuard sets a function that can guard against panic conditions during operations.
func (s *URLStore) SetPanicGuard(fn PanicGuardFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.panicGuard = fn
}

// Save persists a ShortURL entry. If overwrite is false and the code already exists,
// an error is returned.
func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	if err := u.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("store is closed")
	}
	if !overwrite {
		if _, exists := s.urls[u.Code]; exists {
			return fmt.Errorf("code %s already exists", u.Code)
		}
	}
	s.urls[u.Code] = *u
	return nil
}

// SaveWithGuard persists a ShortURL with panic guard checking.
// It returns an error if the panic guard triggers or if the save fails.
func (s *URLStore) SaveWithGuard(u *model.ShortURL, overwrite bool) error {
	if s.panicGuard != nil && s.panicGuard(u.Code, u.RawURL) {
		return fmt.Errorf("panic guard triggered for %s", u.Code)
	}
	return s.Save(u, overwrite)
}

// Get retrieves a ShortURL by its code.
func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, fmt.Errorf("store is closed")
	}
	u, exists := s.urls[code]
	if !exists {
		return nil, fmt.Errorf("code %s not found", code)
	}
	return &u, nil
}

// RawSnapshot returns a raw snapshot of all stored URLs for diagnostic purposes.
func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot := make(map[string]model.ShortURL, len(s.urls))
	for k, v := range s.urls {
		snapshot[k] = v
	}
	return snapshot
}

// AccessLogStore stores access logs for redirects.
type AccessLogStore struct {
	cfg     *config.Config
	mu      sync.Mutex
	logs    []AccessLogEntry
	open    bool
	closed  bool
}

// AccessLogEntry represents a single access log entry.
type AccessLogEntry struct {
	Code      string
	RawURL    string
	IPAddress string
	Timestamp time.Time
	Status    int
}

// NewAccessLogStore creates a new AccessLogStore.
func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	return &AccessLogStore{
		cfg:  cfg,
		logs: make([]AccessLogEntry, 0),
	}, nil
}

// Open initializes the access log store for writing.
func (s *AccessLogStore) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.open = true
	s.closed = false
	return nil
}

// Close releases resources held by the AccessLogStore.
func (s *AccessLogStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.open = false
	return nil
}

// Append adds an access log entry to the store.
func (s *AccessLogStore) Append(entry AccessLogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("access log store is closed")
	}
	if !s.open {
		return fmt.Errorf("access log store is not open")
	}
	s.logs = append(s.logs, entry)
	return nil
}

// Len returns the number of log entries.
func (s *AccessLogStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.logs)
}
