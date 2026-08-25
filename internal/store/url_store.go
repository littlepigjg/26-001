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

type PanicGuardFn func(code, rawURL string) bool

type URLStore struct {
	mu         sync.RWMutex
	data       map[string]model.ShortURL
	panicGuard PanicGuardFn
	cfg        *config.Config
	loaded     bool
}

func NewURLStore(cfg *config.Config) (*URLStore, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	return &URLStore{
		data: make(map[string]model.ShortURL),
		cfg:  cfg,
	}, nil
}

func (s *URLStore) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loaded = true
	return nil
}

func (s *URLStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[string]model.ShortURL)
	s.loaded = false
	return nil
}

func (s *URLStore) SetPanicGuard(fn PanicGuardFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.panicGuard = fn
}

func generateCode(rawURL string) string {
	h := md5.Sum([]byte(rawURL + time.Now().String()))
	return hex.EncodeToString(h[:])[:8]
}

func (s *URLStore) Save(u *model.ShortURL, overwrite bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.panicGuard != nil {
		if s.panicGuard(u.Code, u.RawURL) {
			panic("panic guard triggered")
		}
	}

	if _, exists := s.data[u.Code]; exists && !overwrite {
		return model.NewAppError(model.ErrCodeAlreadyExists, fmt.Sprintf("code %s already exists", u.Code))
	}

	s.data[u.Code] = *u
	return nil
}

func (s *URLStore) Get(code string) (*model.ShortURL, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, exists := s.data[code]
	if !exists {
		return nil, model.NewAppError(model.ErrCodeNotFound, fmt.Sprintf("short URL with code %s not found", code))
	}
	result := u
	return &result, nil
}

func (s *URLStore) RawSnapshot() map[string]model.ShortURL {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]model.ShortURL, len(s.data))
	for k, v := range s.data {
		result[k] = v
	}
	return result
}
