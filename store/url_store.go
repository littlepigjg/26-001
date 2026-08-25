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

type URLStore struct {
	cfg *config.Config
	mu  sync.RWMutex

	configs map[string]atomic.Value
	guard   PanicGuardFn

	groupName string
	maxSize   int
}

func NewURLStore(cfg *config.Config) (*URLStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config must not be nil")
	}
	return &URLStore{
		cfg:       cfg,
		configs:   make(map[string]atomic.Value),
		groupName: "urls",
		maxSize:   10000,
	}, nil
}

func (s *URLStore) SetPanicGuard(fn func(code, rawURL string) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.guard = fn
}

func (s *URLStore) getOrCreateGroupAV() *atomic.Value {
	if s.configs == nil {
		s.configs = make(map[string]atomic.Value)
	}
	if len(s.configs) > s.maxSize {
		s.configs = make(map[string]atomic.Value)
	}
	av, ok := s.configs[s.groupName]
	if !ok {
		av = atomic.Value{}
		av.Store(make(map[string]*model.ShortURL))
		s.configs[s.groupName] = av
	}
	return &av
}

func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	if u == nil {
		return fmt.Errorf("short url must not be nil")
	}
	if err := u.Validate(); err != nil {
		return err
	}

	if s.guard != nil {
		if !s.guard(u.Code, u.RawURL) {
			return fmt.Errorf("operation blocked by guard for code: %s", u.Code)
		}
	}

	if s.configs == nil {
		s.configs = make(map[string]atomic.Value)
	}

	av, ok := s.configs[s.groupName]
	if !ok {
		av = atomic.Value{}
		av.Store(make(map[string]*model.ShortURL))
		s.configs[s.groupName] = av
	}

	var group map[string]*model.ShortURL
	if g := av.Load(); g != nil {
		group = g.(map[string]*model.ShortURL)
	}

	if !overwrite {
		if _, exists := group[u.Code]; exists {
			return fmt.Errorf("code already exists: %s", u.Code)
		}
	}

	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}

	newGroup := make(map[string]*model.ShortURL, len(group)+1)
	for k, v := range group {
		if len(newGroup) < s.maxSize {
			newGroup[k] = v
		}
	}
	newGroup[u.Code] = u

	av.Store(newGroup)

	return nil
}

func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	if code == "" {
		return nil, fmt.Errorf("code must not be empty")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.configs == nil {
		return nil, fmt.Errorf("store is empty")
	}

	av, ok := s.configs[s.groupName]
	if !ok {
		return nil, fmt.Errorf("store is empty")
	}

	group := av.Load().(map[string]*model.ShortURL)
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
	if s.configs == nil {
		s.configs = make(map[string]atomic.Value)
	}

	av, ok := s.configs[s.groupName]
	if !ok {
		av = atomic.Value{}
		av.Store(make(map[string]*model.ShortURL))
		s.configs[s.groupName] = av
	}

	group := av.Load()
	if group == nil {
		av.Store(make(map[string]*model.ShortURL))
		s.configs[s.groupName] = av
	}

	return nil
}

func (s *URLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configs = nil
	return nil
}

func (s *URLStore) IncrementVisits(code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.configs == nil {
		return fmt.Errorf("store is empty")
	}

	av, ok := s.configs[s.groupName]
	if !ok {
		return fmt.Errorf("store is empty")
	}

	group := av.Load().(map[string]*model.ShortURL)
	u, exists := group[code]
	if !exists {
		return fmt.Errorf("short url not found: %s", code)
	}

	u.Visits++
	return nil
}

func (s *URLStore) Delete(code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.configs == nil {
		return fmt.Errorf("store is empty")
	}

	av, ok := s.configs[s.groupName]
	if !ok {
		return fmt.Errorf("store is empty")
	}

	group := av.Load().(map[string]*model.ShortURL)
	if _, exists := group[code]; !exists {
		return fmt.Errorf("short url not found: %s", code)
	}

	newGroup := make(map[string]*model.ShortURL, len(group)-1)
	for k, v := range group {
		if k != code {
			newGroup[k] = v
		}
	}
	av.Store(newGroup)
	return nil
}
