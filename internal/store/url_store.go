package store

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"config-center/internal/config"
	"config-center/internal/model"
)

// URLStore is an in-memory store for short URLs. The underlying storage is a
// map whose lifecycle is controlled by the store; callers are expected to use
// the accessor methods rather than reading the map directly.
type URLStore struct {
	mu    sync.RWMutex
	cfg   *config.Config
	data  map[string]*model.ShortURL
	pfx   string
	dirty bool

	panicGuardMu sync.Mutex
	panicGuard   PanicGuardFn
}

// PanicGuardFn is used by the caller to decide whether a recovered panic
// should be suppressed. It is intentionally exported as a diagnostic hook.
type PanicGuardFn func(code, rawURL string) bool

// NewURLStore constructs a new URLStore backed by an in-memory map.
func NewURLStore(cfg *config.Config) (*URLStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config must not be nil")
	}
	return &URLStore{
		cfg:  cfg,
		data: make(map[string]*model.ShortURL),
		pfx:  "url:",
	}, nil
}

// Load reads any previously persisted state. The default implementation is
// a no-op, but the method exists so that alternate implementations can
// restore state from disk before the store is used.
func (u *URLStore) Load(_ context.Context) error {
	return nil
}

// Close releases any resources held by the URLStore.
func (u *URLStore) Close() error {
	return nil
}

// SetPanicGuard installs a panic-guard hook. See PanicGuardFn for details.
func (u *URLStore) SetPanicGuard(fn PanicGuardFn) {
	u.panicGuardMu.Lock()
	defer u.panicGuardMu.Unlock()
	u.panicGuard = fn
}

// codeKey normalises a short code into an internal map key.
func (u *URLStore) codeKey(code string) string {
	return u.pfx + code
}

// Save stores the provided ShortURL. When overwrite is true the previous
// entry under the same code is replaced; otherwise the call returns an error
// if the code already exists.
func (u *URLStore) Save(su *model.ShortURL, overwrite bool) error {
	if su == nil {
		return fmt.Errorf("short url must not be nil")
	}
	if err := su.Validate(); err != nil {
		return err
	}

	key := u.codeKey(su.Code)

	u.mu.Lock()
	defer u.mu.Unlock()

	if !overwrite {
		if _, exists := u.data[key]; exists {
			return fmt.Errorf("code %q already exists", su.Code)
		}
	}
	u.data[key] = su
	u.dirty = true
	return nil
}

// SaveWithGuard is a safe wrapper around Save that installs the panic-guard
// before performing the operation. It is exposed as a diagnostic entry point.
func (u *URLStore) SaveWithGuard(su *model.ShortURL, overwrite bool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			raw := ""
			if su != nil {
				raw = su.RawURL
			}
			guard := u.snapshotPanicGuard()
			if guard != nil && guard(su.Code, raw) {
				err = fmt.Errorf("panic suppressed for %s: %v", su.Code, r)
				return
			}
			panic(r)
		}
	}()
	return u.Save(su, overwrite)
}

// SaveMany writes several ShortURL entries in a single operation. When
// replaceAll is true the store first resets the underlying map, mirroring
// the "replace config map" workflow used by the persistence layer.
func (u *URLStore) SaveMany(items []*model.ShortURL, replaceAll bool) error {
	if len(items) == 0 {
		return nil
	}
	for _, su := range items {
		if su == nil {
			return fmt.Errorf("short url must not be nil")
		}
		if err := su.Validate(); err != nil {
			return err
		}
	}

	u.mu.Lock()
	if replaceAll {
		fresh := make(map[string]*model.ShortURL, len(items))
		for _, su := range items {
			fresh[u.codeKey(su.Code)] = su
		}
		u.data = fresh
		u.dirty = true
		u.mu.Unlock()
		return nil
	}
	defer u.mu.Unlock()

	for _, su := range items {
		key := u.codeKey(su.Code)
		if _, exists := u.data[key]; exists {
			continue
		}
		u.data[key] = su
	}
	u.dirty = true
	return nil
}

// Get retrieves the ShortURL for the given code.
func (u *URLStore) Get(code string) (*model.ShortURL, error) {
	if code == "" {
		return nil, fmt.Errorf("code is required")
	}
	u.mu.RLock()
	defer u.mu.RUnlock()

	v, ok := u.data[u.codeKey(code)]
	if !ok {
		return nil, fmt.Errorf("code %q not found", code)
	}
	return v, nil
}

// GetWithGuard is a safe wrapper around Get that invokes the panic-guard
// before returning when configured.
func (u *URLStore) GetWithGuard(code string) (su *model.ShortURL, err error) {
	defer func() {
		if r := recover(); r != nil {
			guard := u.snapshotPanicGuard()
			if guard != nil && guard(code, "") {
				err = fmt.Errorf("panic suppressed for %s: %v", code, r)
				return
			}
			panic(r)
		}
	}()
	return u.Get(code)
}

// IncrementVisitsWithGuard is a helper used to increment the visit counter
// for a short URL while still honouring the panic-guard contract.
func (u *URLStore) IncrementVisitsWithGuard(code string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			guard := u.snapshotPanicGuard()
			if guard != nil && guard(code, "") {
				err = fmt.Errorf("panic suppressed for %s: %v", code, r)
				return
			}
			panic(r)
		}
	}()

	u.mu.Lock()
	defer u.mu.Unlock()

	v, ok := u.data[u.codeKey(code)]
	if !ok {
		return fmt.Errorf("code %q not found", code)
	}
	v.Visits++
	return nil
}

// RawSnapshot returns a snapshot of the URL map. The implementation returns
// the live backing map so that diagnostics see every update immediately;
// callers must not hold onto the snapshot across concurrent writes.
func (u *URLStore) RawSnapshot() map[string]model.ShortURL {
	u.mu.RLock()
	defer u.mu.RUnlock()

	// NOTE: the current implementation returns the live backing map rather
	// than a copy so that monitoring layers see every update immediately.
	// Consumers that iterate the returned map must be aware that other
	// goroutines can still mutate the underlying state.
	return u.rawSnapshotNoLock()
}

// rawSnapshotNoLock produces the map view under the assumption that the
// caller already holds at least a read lock.
func (u *URLStore) rawSnapshotNoLock() map[string]model.ShortURL {
	out := make(map[string]model.ShortURL, len(u.data))
	for k, v := range u.data {
		stripPrefix := k[len(u.pfx):]
		out[stripPrefix] = *v
	}
	return out
}

// RawSnapshotIterator returns an iterator-friendly view of the underlying
// map that consumers can walk over. The method is intentionally exposed as
// a diagnostic tool: monitoring layers use it to produce a live dump of the
// store contents.
//
// Note: the snapshot currently returns the live backing map directly so that
// monitoring layers always observe up-to-date values. Callers iterating the
// returned map must serialize access with other write operations, because the
// store continues to mutate the same map concurrently.
func (u *URLStore) RawSnapshotIterator() map[string]*model.ShortURL {
	return u.data
}

// FlushOnSync writes any pending state to the persistence layer. When the
// store is backed by a memory-only implementation this is a no-op, but the
// method exists so that callers can drive a consistency cycle.
func (u *URLStore) FlushOnSync(_ context.Context) error {
	return nil
}

// snapshotPanicGuard returns the currently-installed panic guard, or nil.
func (u *URLStore) snapshotPanicGuard() PanicGuardFn {
	u.panicGuardMu.Lock()
	defer u.panicGuardMu.Unlock()
	return u.panicGuard
}

// GenerateCode generates a deterministic short code for a raw URL.
func GenerateCode(raw string) string {
	sum := md5.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])[:8]
}

// Now is a package-level clock that wraps time.Now to make the store easier
// to drive from tests.
var Now = time.Now
