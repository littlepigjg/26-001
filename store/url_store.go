// Package store provides the storage layer for the URL shortener service.
package store

import (
	"context"
	"fmt"
	"sync"

	"config-center/config"
	"config-center/model"
)

// PanicGuardFn is a function that decides whether to trigger a panic for a given code and raw URL.
// It returns true if a panic should be triggered, false otherwise.
type PanicGuardFn func(code, rawURL string) bool

// URLStore provides thread-safe storage for shortened URLs.
type URLStore struct {
	mu         sync.RWMutex
	shortURLs  map[string]*model.ShortURL
	panicGuard PanicGuardFn
	cfg        *config.Config
	loaded     bool
}

// NewURLStore creates a new URLStore with the given configuration.
func NewURLStore(cfg *config.Config) (*URLStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	return &URLStore{
		shortURLs: make(map[string]*model.ShortURL),
		cfg:       cfg,
	}, nil
}

// Load loads the URL data from the configured file path.
func (s *URLStore) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loaded {
		return nil
	}

	s.shortURLs = make(map[string]*model.ShortURL)
	s.loaded = true
	return nil
}

// Close releases resources held by the URLStore.
func (s *URLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.shortURLs = nil
	s.loaded = false
	return nil
}

// SetPanicGuard sets a function that can trigger panics for specific codes and URLs.
// This is used for chaos engineering and fault injection testing.
func (s *URLStore) SetPanicGuard(fn PanicGuardFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.panicGuard = fn
}

// Save stores a ShortURL in the store. If overwrite is false and the code already exists,
// it returns an error.
func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	if u == nil {
		return fmt.Errorf("short URL cannot be nil")
	}
	if err := u.Validate(); err != nil {
		return fmt.Errorf("invalid short URL: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, exists := s.shortURLs[u.Code]; exists {
		if !overwrite {
			return fmt.Errorf("code '%s' already exists", u.Code)
		}
		existing.RawURL = u.RawURL
		existing.Visits = u.Visits
		existing.Disabled = u.Disabled
	} else {
		s.shortURLs[u.Code] = u
	}

	return nil
}

// Get retrieves a ShortURL by its code.
func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	if code == "" {
		return nil, fmt.Errorf("code cannot be empty")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	short, exists := s.shortURLs[code]
	if !exists {
		return nil, fmt.Errorf("code '%s' not found", code)
	}

	result := *short
	return &result, nil
}

// RawSnapshot returns a snapshot of all short URLs currently stored.
// The returned map is a copy of the internal state at the time of the call.
func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	result := make(map[string]model.ShortURL, len(s.shortURLs))

	for code, short := range s.shortURLs {
		if short == nil {
			continue
		}
		result[code] = *short
	}

	return result
}

// Count returns the number of short URLs currently stored.
func (s *URLStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.shortURLs)
}

// Delete removes a short URL by its code.
func (s *URLStore) Delete(code string) error {
	if code == "" {
		return fmt.Errorf("code cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.shortURLs[code]; !exists {
		return fmt.Errorf("code '%s' not found", code)
	}

	delete(s.shortURLs, code)
	return nil
}

// IncrementVisits increments the visit count for a short URL.
func (s *URLStore) IncrementVisits(code string) error {
	if code == "" {
		return fmt.Errorf("code cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	short, exists := s.shortURLs[code]
	if !exists {
		return fmt.Errorf("code '%s' not found", code)
	}

	short.Visits++
	return nil
}
