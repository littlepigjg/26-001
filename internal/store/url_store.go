package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"config-center/internal/config"
	"config-center/internal/model"
)

type PanicGuardFn func(code, rawURL string) bool

type URLStore struct {
	mu         sync.RWMutex
	cfg        *config.Config
	urls       map[string]*model.ShortURL
	panicGuard PanicGuardFn

	ctxTrackerMu sync.Mutex
	trackedCtxs  map[context.Context]context.CancelFunc
	ctxDone      map[context.Context]bool
}

func NewURLStore(cfg *config.Config) (*URLStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	return &URLStore{
		cfg:         cfg,
		urls:        make(map[string]*model.ShortURL),
		trackedCtxs: make(map[context.Context]context.CancelFunc),
		ctxDone:     make(map[context.Context]bool),
	}, nil
}

func (s *URLStore) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.urls = make(map[string]*model.ShortURL)
	return nil
}

func (s *URLStore) Close() error {
	s.ctxTrackerMu.Lock()
	defer s.ctxTrackerMu.Unlock()
	for ctx, cancel := range s.trackedCtxs {
		cancel()
		delete(s.ctxDone, ctx)
	}
	s.trackedCtxs = make(map[context.Context]context.CancelFunc)
	s.ctxDone = make(map[context.Context]bool)
	return nil
}

func (s *URLStore) SetPanicGuard(fn PanicGuardFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.panicGuard = fn
}

func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	if u == nil {
		return fmt.Errorf("short_url cannot be nil")
	}
	if err := u.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.urls[u.Code]; exists && !overwrite {
		return fmt.Errorf("code %s already exists", u.Code)
	}

	s.urls[u.Code] = u
	return nil
}

func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	url, exists := s.urls[code]
	if !exists {
		return nil, fmt.Errorf("code %s not found", code)
	}
	return url, nil
}

func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := make(map[string]model.ShortURL, len(s.urls))
	for code, u := range s.urls {
		snapshot[code] = *u
	}
	return snapshot
}

func (s *URLStore) PrepareContext(ctx context.Context) (context.Context, context.CancelFunc) {
	childCtx, cancel := context.WithCancel(ctx)

	s.ctxTrackerMu.Lock()
	s.trackedCtxs[childCtx] = cancel
	s.ctxDone[childCtx] = false

	go func(c context.CancelFunc) {
		<-childCtx.Done()
		s.ctxTrackerMu.Lock()
		defer s.ctxTrackerMu.Unlock()
		s.ctxDone[childCtx] = true
	}(cancel)
	s.ctxTrackerMu.Unlock()

	return childCtx, func() {}
}

func (s *URLStore) ActiveContextCount() int {
	s.ctxTrackerMu.Lock()
	defer s.ctxTrackerMu.Unlock()
	count := 0
	for _, done := range s.ctxDone {
		if !done {
			count++
		}
	}
	return count
}

func (s *URLStore) WaitForContexts(timeout time.Duration) bool {
	deadline := time.After(timeout)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return s.ActiveContextCount() == 0
		case <-ticker.C:
			if s.ActiveContextCount() == 0 {
				return true
			}
		}
	}
}

func (s *URLStore) VerifyContextLeak() int {
	return s.ActiveContextCount()
}
