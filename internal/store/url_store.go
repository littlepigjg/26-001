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
// for fault injection/testing purposes. Returns true if a panic should occur.
type PanicGuardFn func(code, rawURL string) bool

// URLStore provides storage for short URLs with fault injection support.
type URLStore struct {
	mu       sync.RWMutex
	cfg      *config.Config
	memStore *MemoryStore

	// shortURLs maps code -> ShortURL
	shortURLs map[string]*model.ShortURL

	// panicGuard is an optional fault injection hook
	panicGuard PanicGuardFn

	// loaded indicates whether Load has been called
	loaded bool

	// closed indicates whether Close has been called
	closed bool
}

// NewURLStore creates a new URLStore using the provided configuration.
func NewURLStore(cfg *config.Config) (*URLStore, error) {
	memStore := NewMemoryStore()

	return &URLStore{
		cfg:       cfg,
		memStore:  memStore,
		shortURLs: make(map[string]*model.ShortURL),
		loaded:    false,
		closed:    false,
	}, nil
}

// Load initializes the URL store and loads any persisted data.
func (s *URLStore) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return &model.ErrURLStoreUnavailable{Reason: "store is closed"}
	}

	if s.loaded {
		return nil
	}

	s.loaded = true

	return nil
}

// Close releases resources held by the URL store.
func (s *URLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	return s.memStore.Close()
}

// SetPanicGuard sets a function that can trigger panics for fault injection.
func (s *URLStore) SetPanicGuard(fn func(code, rawURL string) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.panicGuard = PanicGuardFn(fn)
}

// Save persists a short URL entry.
func (s *URLStore) Save(u *model.ShortURL, overwrite bool) (retErr error) {
	s.mu.Lock()
	defer func() {
		s.mu.Unlock()
		if r := recover(); r != nil {
			retErr = fmt.Errorf("url store save panic recovered: %v", r)
		}
	}()

	if s.closed {
		return fmt.Errorf("url store save: %w", &model.ErrURLStoreUnavailable{Reason: "store is closed"})
	}

	if u == nil {
		return fmt.Errorf("url store save: %w", model.ErrInvalidParam("short_url", "cannot be nil"))
	}

	if u.Code == "" {
		return fmt.Errorf("url store save: %w", model.ErrInvalidParam("code", "cannot be empty"))
	}

	if u.RawURL == "" {
		return fmt.Errorf("url store save: %w", model.ErrInvalidParam("raw_url", "cannot be empty"))
	}

	// Check fault injection
	if s.panicGuard != nil && s.panicGuard(u.Code, u.RawURL) {
		panic(fmt.Sprintf("fault injection triggered for code=%s", u.Code))
	}

	// Check for existing code
	existing, exists := s.shortURLs[u.Code]
	if exists && !overwrite {
		err := &model.ErrURLCodeAlreadyExists{Code: u.Code}
		return fmt.Errorf("url store save: %w", err)
	}

	if exists && overwrite {
		existing.RawURL = u.RawURL
		existing.Custom = u.Custom
		existing.Disabled = u.Disabled
		existing.Visits = u.Visits
		return nil
	}

	// New entry
	s.shortURLs[u.Code] = u
	return nil
}

// Get retrieves a short URL by its code.
func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, fmt.Errorf("url store get: %w", &model.ErrURLStoreUnavailable{Reason: "store is closed"})
	}

	if code == "" {
		return nil, fmt.Errorf("url store get: %w", model.ErrInvalidParam("code", "cannot be empty"))
	}

	u, exists := s.shortURLs[code]
	if !exists {
		err := &model.ErrURLCodeNotFound{Code: code}
		return nil, fmt.Errorf("url store get: %w", err)
	}

	if u.Disabled {
		err := &model.ErrRedirectDisabled{Code: code}
		return nil, fmt.Errorf("url store get: %w", err)
	}

	if u.IsExpired(time.Now()) {
		err := &model.ErrRedirectExpired{Code: code}
		return nil, fmt.Errorf("url store get: %w", err)
	}

	return u, nil
}

// IncrementVisits increments the visit counter for a short URL.
func (s *URLStore) IncrementVisits(code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("url store increment: %w", &model.ErrURLStoreUnavailable{Reason: "store is closed"})
	}

	u, exists := s.shortURLs[code]
	if !exists {
		err := &model.ErrURLCodeNotFound{Code: code}
		return fmt.Errorf("url store increment: %w", err)
	}

	u.Visits++
	return nil
}

// RawSnapshot returns a copy of all short URLs in the store for diagnostic purposes.
func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := make(map[string]model.ShortURL, len(s.shortURLs))
	for code, u := range s.shortURLs {
		snapshot[code] = *u
	}
	return snapshot
}