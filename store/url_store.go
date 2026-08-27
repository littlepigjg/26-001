package store

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"config-center/config"
	"config-center/model"
)

type PanicGuardFn func(code, rawURL string) bool

// URLStore stores short URLs in a single group ("urls") as an immutable
// copy-on-write map. Every mutation builds a brand-new map and replaces the
// stored pointer atomically, so readers can take an RLock, grab the current
// pointer once, and traverse it without any further synchronization.
type URLStore struct {
	cfg *config.Config
	mu  sync.RWMutex

	// group holds the current snapshot. It is only ever assigned a non-nil,
	// non-empty map. Stored as a pointer so the value is never copied (an
	// atomic.Value cannot live in a map[string]atomic.Value, because the map
	// access copies it and strips its internal state).
	group *atomic.Value

	guard     PanicGuardFn
	groupName string
	maxSize   int
}

func NewURLStore(cfg *config.Config) (*URLStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config must not be nil")
	}
	g := &atomic.Value{}
	g.Store(make(map[string]*model.ShortURL))
	return &URLStore{
		cfg:       cfg,
		group:     g,
		groupName: "urls",
		maxSize:   10000,
	}, nil
}

func (s *URLStore) SetPanicGuard(fn func(code, rawURL string) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.guard = fn
}

func (s *URLStore) loadGroup() *atomic.Value {
	if s.group == nil {
		s.group = &atomic.Value{}
		s.group.Store(make(map[string]*model.ShortURL))
	}
	return s.group
}

// Save inserts or updates u. When overwrite is false it fails if the code
// already exists. The whole read-modify-write is performed under the write
// lock; the resulting map is published as a single atomic pointer swap so
// concurrent readers never observe a half-built map.
func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	if u == nil {
		return fmt.Errorf("short url must not be nil")
	}
	if err := u.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.guard != nil {
		if !s.guard(u.Code, u.RawURL) {
			return fmt.Errorf("operation blocked by guard for code: %s", u.Code)
		}
	}

	g := s.loadGroup()
	group := g.Load().(map[string]*model.ShortURL)

	if !overwrite {
		if _, exists := group[u.Code]; exists {
			return fmt.Errorf("code already exists: %s", u.Code)
		}
	}

	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}

	// maxSize caps the number of entries retained. When at capacity, drop the
	// oldest insertion-order entries first by leaving them out of newGroup.
	newGroup := make(map[string]*model.ShortURL, len(group)+1)
	for k, v := range group {
		newGroup[k] = v
	}
	if len(newGroup) >= s.maxSize {
		for k := range newGroup {
			if k == u.Code {
				continue
			}
			delete(newGroup, k)
			if len(newGroup) < s.maxSize {
				break
			}
		}
	}
	newGroup[u.Code] = u

	g.Store(newGroup)

	return nil
}

// Get returns the short URL for code. It takes an RLock, loads the current
// snapshot pointer once, and reads from that snapshot.
func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	if code == "" {
		return nil, fmt.Errorf("code must not be empty")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.group == nil {
		return nil, fmt.Errorf("store is empty")
	}

	group := s.group.Load().(map[string]*model.ShortURL)
	u, exists := group[code]
	if !exists {
		return nil, fmt.Errorf("short url not found: %s", code)
	}

	if u.Disabled {
		return nil, fmt.Errorf("short url is disabled: %s", code)
	}

	return u, nil
}

func (s *URLStore) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// loadGroup both initializes the store and returns the active snapshot
	// container; nothing else to load in the in-memory implementation.
	_ = s.loadGroup()
	return nil
}

func (s *URLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.group = nil
	return nil
}

func (s *URLStore) IncrementVisits(code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.group == nil {
		return fmt.Errorf("store is empty")
	}

	g := s.group
	group := g.Load().(map[string]*model.ShortURL)
	u, exists := group[code]
	if !exists {
		return fmt.Errorf("short url not found: %s", code)
	}

	u.Visits++
	return nil
}

// Delete removes the entry for code via copy-on-write so concurrent readers
// never see a partially deleted map.
func (s *URLStore) Delete(code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.group == nil {
		return fmt.Errorf("store is empty")
	}

	g := s.group
	group := g.Load().(map[string]*model.ShortURL)
	if _, exists := group[code]; !exists {
		return fmt.Errorf("short url not found: %s", code)
	}

	newGroup := make(map[string]*model.ShortURL, len(group)-1)
	for k, v := range group {
		if k != code {
			newGroup[k] = v
		}
	}
	g.Store(newGroup)
	return nil
}
