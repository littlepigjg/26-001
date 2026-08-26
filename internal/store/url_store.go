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
	urls       map[string]*model.ShortURL
	panicGuard PanicGuardFn
	cfg        *config.Config
	loaded     bool
}

type AccessLogStore struct {
	mu     sync.RWMutex
	logs   []model.AuditLog
	cfg    *config.Config
	opened bool
}

func NewURLStore(cfg *config.Config) (*URLStore, error) {
	if cfg == nil {
		cfg = config.Default()
	}
	return &URLStore{
		urls: make(map[string]*model.ShortURL),
		cfg:  cfg,
	}, nil
}

func NewAccessLogStore(cfg *config.Config) (*AccessLogStore, error) {
	if cfg == nil {
		cfg = config.Default()
	}
	return &AccessLogStore{
		logs: make([]model.AuditLog, 0),
		cfg:  cfg,
	}, nil
}

func (s *URLStore) Load(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loaded = true
	return nil
}

func (s *URLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.urls = make(map[string]*model.ShortURL)
	s.loaded = false
	return nil
}

func (s *URLStore) SetPanicGuard(fn PanicGuardFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.panicGuard = fn
}

func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	if u == nil {
		return fmt.Errorf("short URL cannot be nil")
	}
	if err := u.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.urls[u.Code]; exists && !overwrite {
		return fmt.Errorf("code '%s' already exists", u.Code)
	}

	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}

	s.urls[u.Code] = u
	return nil
}

func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	url, exists := s.urls[code]
	if !exists {
		return nil, fmt.Errorf("code '%s' not found", code)
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

func (s *AccessLogStore) Open(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opened = true
	return nil
}

func (s *AccessLogStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opened = false
	return nil
}

func (s *AccessLogStore) WriteLog(log model.AuditLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.opened {
		return fmt.Errorf("access log store is not opened")
	}
	s.logs = append(s.logs, log)
	return nil
}

func (s *AccessLogStore) GetLogs() []model.AuditLog {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]model.AuditLog, len(s.logs))
	copy(result, s.logs)
	return result
}

type shortURLGen struct {
	mu      sync.Mutex
	counter int
}

var urlGen = &shortURLGen{}

func (g *shortURLGen) nextCode() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.counter++
	return fmt.Sprintf("s%06d", g.counter)
}

func GenerateCode() string {
	return urlGen.nextCode()
}
