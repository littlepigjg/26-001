package service

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"config-center/config"
	"config-center/model"
	"config-center/store"
)

type URLService struct {
	cfg   *config.Config
	store *store.URLStore
}

func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config must not be nil")
	}
	if s == nil {
		return nil, fmt.Errorf("url store must not be nil")
	}
	return &URLService{
		cfg:   cfg,
		store: s,
	}, nil
}

func (s *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if req == nil {
		return nil, fmt.Errorf("create request must not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	code := req.CustomCode
	isCustom := false
	if code == "" {
		code = generateCode()
	} else {
		isCustom = true
	}

	shortURL := &model.ShortURL{
		Code:      code,
		RawURL:    req.RawURL,
		CreatedAt: time.Now(),
		Visits:    0,
		Custom:    isCustom,
		Disabled:  false,
		MaxVisits: req.MaxVisits,
	}

	if err := s.store.Save(shortURL, false); err != nil {
		return nil, fmt.Errorf("failed to save short url: %w", err)
	}

	return shortURL, nil
}

func generateCode() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
