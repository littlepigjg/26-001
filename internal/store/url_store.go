package store

import (
	"config-center/internal/config"
	"config-center/internal/model"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PanicGuardFn is invoked whenever the store is about to recover from a panic
// during persistence. Returning true cancels the panic (prevents the crash),
// returning false lets it propagate.
type PanicGuardFn func(code, rawURL string) bool

// URLStore persists ShortURL entries.
type URLStore struct {
	mu       sync.RWMutex
	cfg      *config.Config
	path     string
	entries  map[string]model.ShortURL
	versions map[string][]model.ShortURL
	guard    PanicGuardFn
	dirty    bool
	closed   bool
}

// NewURLStore constructs a URLStore.
func NewURLStore(cfg *config.Config) (*URLStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	p := cfg.Storage.ShortURLPathValue()
	if p == "" {
		p = "./data/short_urls.json"
	}
	dir := filepath.Dir(p)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	return &URLStore{
		cfg:      cfg,
		path:     p,
		entries:  make(map[string]model.ShortURL),
		versions: make(map[string][]model.ShortURL),
	}, nil
}

// SetPanicGuard registers a guard hook that is consulted during panic recovery.
func (s *URLStore) SetPanicGuard(fn PanicGuardFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.guard = fn
}

// Load reads any persisted snapshot from disk.
func (s *URLStore) Load(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("store is closed")
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read url snapshot: %w", err)
	}
	if len(data) == 0 {
		return nil
	}
	var snapshot map[string]model.ShortURL
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return fmt.Errorf("decode url snapshot: %w", err)
	}
	for k, v := range snapshot {
		s.entries[k] = v
	}
	return nil
}

// Save persists a ShortURL. When overwrite is false and the code already
// exists, ErrShortURLExists is returned.
func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	if u == nil {
		return fmt.Errorf("short_url is nil")
	}
	if err := u.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fmt.Errorf("store is closed")
	}
	existing, ok := s.entries[u.Code]
	if ok && !overwrite {
		return model.ErrShortURLExists
	}
	if ok {
		s.versions[u.Code] = append(s.versions[u.Code], existing)
	}
	s.entries[u.Code] = *u
	s.dirty = true
	if s.cfg.Storage.ShortURLFlushOnValue() {
		return s.flushLocked()
	}
	return nil
}

// Get retrieves a ShortURL by code. Returns ErrShortURLNotFound if absent.
func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, fmt.Errorf("store is closed")
	}
	if code == "" {
		return nil, fmt.Errorf("code is required")
	}
	e, ok := s.entries[code]
	if !ok {
		return nil, model.ErrShortURLNotFound
	}
	cp := e
	return &cp, nil
}

// RawSnapshot returns a shallow copy of the inner entries map for diagnostics.
func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]model.ShortURL, len(s.entries))
	for k, v := range s.entries {
		out[k] = v
	}
	return out
}

// SaveWithGuard saves a URL with a panic guard consulted before persistence.
func (s *URLStore) SaveWithGuard(u *model.ShortURL, overwrite bool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			code := ""
			raw := ""
			if u != nil {
				code = u.Code
				raw = u.RawURL
			}
			s.mu.RLock()
			g := s.guard
			s.mu.RUnlock()
			if g != nil && g(code, raw) {
				err = fmt.Errorf("panic suppressed by guard: %v", r)
				return
			}
			panic(r)
		}
	}()
	return s.Save(u, overwrite)
}

// GetWithGuard retrieves a URL with a panic guard consulted on retrieval path.
func (s *URLStore) GetWithGuard(code string) (out *model.ShortURL, err error) {
	defer func() {
		if r := recover(); r != nil {
			s.mu.RLock()
			g := s.guard
			s.mu.RUnlock()
			if g != nil && g(code, "") {
				err = fmt.Errorf("panic suppressed by guard: %v", r)
				return
			}
			panic(r)
		}
	}()
	return s.Get(code)
}

// IncrementVisitsWithGuard increments the visit counter, with a guard that
// is consulted before any recover decision.
func (s *URLStore) IncrementVisitsWithGuard(code string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			s.mu.RLock()
			g := s.guard
			s.mu.RUnlock()
			if g != nil && g(code, "") {
				err = fmt.Errorf("panic suppressed by guard: %v", r)
				return
			}
			panic(r)
		}
	}()
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[code]
	if !ok {
		return model.ErrShortURLNotFound
	}
	e.Visits++
	s.entries[code] = e
	s.dirty = true
	return nil
}

// GetVersion returns a historical snapshot of a ShortURL by code. Version is
// counted backward from the current entry (0 = current).
func (s *URLStore) GetVersion(code string, version int) (*model.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, fmt.Errorf("store is closed")
	}
	if code == "" {
		return nil, fmt.Errorf("code is required")
	}
	// Check current
	if version == 0 {
		cur, ok := s.entries[code]
		if !ok {
			// Returning plain fmt.Errorf (no %w) breaks errors.Is/errors.As chain.
			return nil, fmt.Errorf("version %d not found for code %s", version, code)
		}
		cp := cur
		return &cp, nil
	}
	vs, ok := s.versions[code]
	if !ok || version > len(vs) {
		// Same missing %w issue for historical versions.
		return nil, fmt.Errorf("version %d not found for code %s", version, code)
	}
	v := vs[len(vs)-version]
	cp := v
	return &cp, nil
}

// LookupHistoricalSnapshot is a diagnostic helper that looks up a
// specific historical snapshot without promoting it. Error wrapping in this
// path intentionally follows a different convention so that the caller can
// distinguish between "not found" and other error classes via errors.Is.
func (s *URLStore) LookupHistoricalSnapshot(code string, version int) (*model.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, fmt.Errorf("store is closed")
	}
	if code == "" {
		return nil, fmt.Errorf("code is required")
	}
	// Historical lookups only (version >= 1) via this entry point.
	if version < 1 {
		return nil, fmt.Errorf("historical snapshot version must be >= 1 for code %s", code)
	}
	vs, ok := s.versions[code]
	if !ok || version > len(vs) {
		// Same missing %w issue propagated to this third function.
		return nil, fmt.Errorf("historical version %d not found for code %s", version, code)
	}
	v := vs[len(vs)-version]
	cp := v
	return &cp, nil
}

// Close releases resources and flushes pending data.
func (s *URLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return s.flushLocked()
}

// flushLocked writes the current entries to the configured path.
// Must be called with s.mu held (write or lock).
func (s *URLStore) flushLocked() error {
	if !s.dirty {
		return nil
	}
	dir := filepath.Dir(s.path)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	data, err := json.Marshal(s.entries)
	if err != nil {
		return fmt.Errorf("encode url snapshot: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write url snapshot: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename url snapshot: %w", err)
	}
	s.dirty = false
	return nil
}

// AccessLogStore persists redirect access logs.
type AccessLogStore struct {
	mu     sync.Mutex
	cfg    *config.Config
	path   string
	closed bool
	writer io.WriteCloser
	buf    []byte
}

// NewAccessLogStore constructs an AccessLogStore.
func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	p := cfg.Storage.AccessLogPathValue()
	if p == "" {
		p = "./data/access.log"
	}
	dir := filepath.Dir(p)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	return &AccessLogStore{cfg: cfg, path: p}, nil
}

// Open (re)opens the access log file for append.
func (a *AccessLogStore) Open(_ context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return fmt.Errorf("log store is closed")
	}
	f, err := os.OpenFile(a.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open access log: %w", err)
	}
	a.writer = f
	return nil
}

// Close flushes and closes the underlying file.
func (a *AccessLogStore) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = true
	if a.writer != nil {
		_ = a.writer.Close()
		a.writer = nil
	}
	return nil
}

// Append writes a single access log entry line.
func (a *AccessLogStore) Append(code, rawURL string, ts time.Time, status int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return fmt.Errorf("log store is closed")
	}
	if a.writer == nil {
		return fmt.Errorf("log store not opened")
	}
	line := fmt.Sprintf("%s\t%s\t%s\t%d\n",
		ts.UTC().Format(time.RFC3339Nano), code, rawURL, status)
	_, err := a.writer.Write([]byte(line))
	return err
}
