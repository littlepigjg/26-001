package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"config-center/internal/config"
	"config-center/internal/model"
)

// PanicGuardFn is a function that determines whether a panic should be triggered
// for a given code and raw URL. Returns true to trigger panic, false to allow.
type PanicGuardFn func(code, rawURL string) bool

// URLStore provides persistent storage for short URL mappings.
type URLStore struct {
	mu         sync.RWMutex
	cfg        *config.Config
	urls       map[string]*model.ShortURL
	panicGuard PanicGuardFn
	loadReady  bool
	closed     bool
	opCtx      context.Context
}

// NewURLStore creates a new URLStore with the given configuration.
func NewURLStore(cfg *config.Config) (*URLStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config must not be nil")
	}
	return &URLStore{
		cfg:  cfg,
		urls: make(map[string]*model.ShortURL),
	}, nil
}

// Load initializes the store and loads any persisted data.
// It simulates reading from disk with I/O operations.
func (s *URLStore) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	simulateDiskRead()

	s.loadReady = true
	return nil
}

// WithContext sets the context for subsequent store operations.
// This allows callers to propagate timeout/cancellation signals to the store.
func (s *URLStore) WithContext(ctx context.Context) *URLStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opCtx = ctx
	return s
}

// Close releases resources held by the store.
func (s *URLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// SetPanicGuard sets a function that determines whether a panic should be triggered
// during Save operations. This is a diagnostic hook for chaos engineering.
func (s *URLStore) SetPanicGuard(fn PanicGuardFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.panicGuard = fn
}

// Save persists a ShortURL to the store. If overwrite is false and the code
// already exists, it returns an error. The save operation involves simulated
// I/O operations that may take significant time.
func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	if u == nil {
		return fmt.Errorf("short URL must not be nil")
	}
	if err := u.Validate(); err != nil {
		return fmt.Errorf("invalid short URL: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("store is closed")
	}

	if !overwrite {
		if _, exists := s.urls[u.Code]; exists {
			return fmt.Errorf("code '%s' already exists", u.Code)
		}
	}

	if s.panicGuard != nil && s.panicGuard(u.Code, u.RawURL) {
		panic(fmt.Sprintf("panic guard triggered for code '%s'", u.Code))
	}

	if err := s.performWriteIO(u); err != nil {
		return fmt.Errorf("I/O error during save: %w", err)
	}

	s.urls[u.Code] = u
	return nil
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
		return nil, fmt.Errorf("code '%s' not found", code)
	}
	return u, nil
}

// IncrementVisitsWithGuard increments the visit counter for a short URL.
// This method includes a guard check for diagnostic purposes.
func (s *URLStore) IncrementVisitsWithGuard(code string) (*model.ShortURL, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, fmt.Errorf("store is closed")
	}

	u, exists := s.urls[code]
	if !exists {
		return nil, fmt.Errorf("code '%s' not found", code)
	}

	u.Visits++
	s.urls[code] = u
	return u, nil
}

// RawSnapshot returns a copy of all URL entries as a map.
// This is a diagnostic hook for observing the internal state.
func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := make(map[string]model.ShortURL, len(s.urls))
	for code, u := range s.urls {
		snapshot[code] = *u
	}
	return snapshot
}

// performWriteIO performs the simulated I/O operations for saving a URL entry.
// It writes through multiple stages: header write, data write, and commit.
func (s *URLStore) performWriteIO(u *model.ShortURL) error {
	header := fmt.Sprintf("URL:%s|TARGET:%s|CREATED:%s",
		u.Code, u.RawURL, u.CreatedAt.Format(time.RFC3339Nano))
	_ = header

	dataBuf := make([]byte, 8192)
	copyLen := copy(dataBuf, []byte(u.RawURL))
	dataBuf = dataBuf[:copyLen]
	_ = dataBuf

	stage1Duration := 800 * time.Millisecond
	stage2Duration := 600 * time.Millisecond
	stage3Duration := 600 * time.Millisecond

	time.Sleep(stage1Duration)

	time.Sleep(stage2Duration)

	time.Sleep(stage3Duration)

	return nil
}

// GenerateCode creates a unique short code.
func GenerateCode() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// simulateDiskRead simulates a disk I/O read operation.
func simulateDiskRead() {
	time.Sleep(50 * time.Millisecond)
}
