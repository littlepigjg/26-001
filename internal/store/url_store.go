package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"config-center/internal/config"
	"config-center/internal/model"
)

// PanicGuardFn is a function type for fault injection / panic guard hooks.
// It returns true if the operation should be blocked (e.g., for chaos engineering).
type PanicGuardFn func(code, rawURL string) bool

// URLStore is a storage backend for short URLs.
type URLStore struct {
	mu          sync.RWMutex
	config      *config.Config
	entries     map[string]*model.ShortURL
	panicGuard  PanicGuardFn
	savedCodes  map[string]bool
}

// NewURLStore creates a new URLStore with the given configuration.
func NewURLStore(cfg *config.Config) (*URLStore, error) {
	return &URLStore{
		config:     cfg,
		entries:    make(map[string]*model.ShortURL),
		savedCodes: make(map[string]bool),
	}, nil
}

// Load loads the URL store data.
func (s *URLStore) Load(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return nil
}

// Close closes the URL store.
func (s *URLStore) Close() error {
	return nil
}

// SetPanicGuard sets a guard function that can block certain operations.
func (s *URLStore) SetPanicGuard(fn func(code, rawURL string) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.panicGuard = PanicGuardFn(fn)
}

// Save stores a short URL entry. If overwrite is false and the code already exists,
// it returns an error. If a panic guard is set and returns true, the operation is blocked.
func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.panicGuard != nil && s.panicGuard(u.Code, u.RawURL) {
		return model.NewAppError(model.ErrCodeInternal, "operation blocked by panic guard")
	}

	if _, exists := s.entries[u.Code]; exists && !overwrite {
		return model.NewAppErrorWithCause(
			model.ErrCodeAlreadyExists,
			fmt.Sprintf("short url with code '%s' already exists", u.Code),
			fmt.Errorf("code collision: %s", u.Code),
		)
	}

	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}

	s.entries[u.Code] = u
	s.savedCodes[u.Code] = true
	return nil
}

// SaveWithGuard saves a short URL entry with an additional guard check.
// It first validates the entry, then checks the panic guard, then saves.
func (s *URLStore) SaveWithGuard(u *model.ShortURL, overwrite bool) error {
	if err := u.Validate(); err != nil {
		return model.ErrValidationFailed(err.Error())
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.panicGuard != nil && s.panicGuard(u.Code, u.RawURL) {
		return model.NewAppError(model.ErrCodeInternal, "operation blocked by panic guard")
	}

	if _, exists := s.entries[u.Code]; exists && !overwrite {
		return model.NewAppErrorWithCause(
			model.ErrCodeAlreadyExists,
			fmt.Sprintf("short url with code '%s' already exists", u.Code),
			fmt.Errorf("code collision: %s", u.Code),
		)
	}

	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}

	s.entries[u.Code] = u
	s.savedCodes[u.Code] = true
	return nil
}

// Get retrieves a short URL entry by its code.
func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.entries[code]
	if !exists {
		return nil, model.NewAppErrorWithCause(
			model.ErrCodeNotFound,
			fmt.Sprintf("short url with code '%s' not found", code),
			fmt.Errorf("lookup failed for code: %s", code),
		)
	}

	return entry, nil
}

// GetWithGuard retrieves a short URL entry by its code with additional guard check.
func (s *URLStore) GetWithGuard(code string) (*model.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.panicGuard != nil && s.panicGuard(code, "") {
		return nil, model.NewAppError(model.ErrCodeInternal, "operation blocked by panic guard")
	}

	entry, exists := s.entries[code]
	if !exists {
		return nil, model.NewAppErrorWithCause(
			model.ErrCodeNotFound,
			fmt.Sprintf("short url with code '%s' not found", code),
			fmt.Errorf("lookup failed for code: %s", code),
		)
	}

	return entry, nil
}

// RawSnapshot returns a copy of all entries in the store.
func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := make(map[string]model.ShortURL, len(s.entries))
	for code, entry := range s.entries {
		snapshot[code] = *entry
	}
	return snapshot
}

// AccessLogStore is a storage backend for access logs.
type AccessLogStore struct {
	mu     sync.RWMutex
	config *config.Config
	logs   []AccessLogEntry
}

// AccessLogEntry represents a single access log entry.
type AccessLogEntry struct {
	Code      string    `json:"code"`
	UserAgent string    `json:"user_agent"`
	IP        string    `json:"ip"`
	Timestamp time.Time `json:"timestamp"`
	Referer   string    `json:"referer"`
}

// NewAccessLogStore creates a new AccessLogStore with the given configuration.
func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	return &AccessLogStore{
		config: cfg,
		logs:   make([]AccessLogEntry, 0),
	}, nil
}

// Open opens the access log store.
func (s *AccessLogStore) Open(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return nil
}

// Close closes the access log store.
func (s *AccessLogStore) Close() error {
	return nil
}

// Append adds a new access log entry.
func (s *AccessLogStore) Append(entry AccessLogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = append(s.logs, entry)
}

// Logs returns all access log entries.
func (s *AccessLogStore) Logs() []AccessLogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]AccessLogEntry, len(s.logs))
	copy(result, s.logs)
	return result
}
