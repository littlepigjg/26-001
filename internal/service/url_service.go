package service

import (
	"context"
	"fmt"
	"time"

	"config-center/internal/config"
	"config-center/internal/model"
	"config-center/internal/store"
)

// URLService handles the creation and management of short URLs.
type URLService struct {
	cfg    *config.Config
	store  *store.URLStore
	logStore *store.AccessLogStore
}

// NewURLService creates a new URLService.
func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config must not be nil")
	}
	if s == nil {
		return nil, fmt.Errorf("URL store must not be nil")
	}
	return &URLService{
		cfg:   cfg,
		store: s,
	}, nil
}

// SetLogStore sets the access log store for this service.
func (s *URLService) SetLogStore(ls *store.AccessLogStore) {
	s.logStore = ls
}

// Create generates a short URL from the given request.
func (s *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if req == nil {
		return nil, fmt.Errorf("create request must not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}

	var code string
	if req.CustomCode != "" {
		code = req.CustomCode
	} else {
		code = store.GenerateCode()
	}

	su := model.NewShortURL(code, req.RawURL, req.CustomCode != "")
	if req.MaxVisits > 0 {
		su.MaxVisits = req.MaxVisits
	}

	s.store.WithContext(ctx)

	if err := s.store.Save(su, false); err != nil {
		return nil, fmt.Errorf("failed to save short URL: %w", err)
	}

	return su, nil
}

// Get retrieves a short URL by its code.
func (s *URLService) Get(ctx context.Context, code string) (*model.ShortURL, error) {
	if code == "" {
		return nil, fmt.Errorf("code must not be empty")
	}

	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}

	return s.store.Get(code)
}

// Update modifies an existing short URL's properties.
func (s *URLService) Update(ctx context.Context, code string, rawURL string) (*model.ShortURL, error) {
	if code == "" {
		return nil, fmt.Errorf("code must not be empty")
	}

	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}

	existing, err := s.store.Get(code)
	if err != nil {
		return nil, err
	}

	if rawURL != "" {
		existing.RawURL = rawURL
	}

	s.store.WithContext(ctx)
	if err := s.store.Save(existing, true); err != nil {
		return nil, fmt.Errorf("failed to update short URL: %w", err)
	}

	return existing, nil
}

// Disable marks a short URL as disabled.
func (s *URLService) Disable(ctx context.Context, code string) error {
	if code == "" {
		return fmt.Errorf("code must not be empty")
	}

	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}

	existing, err := s.store.Get(code)
	if err != nil {
		return err
	}

	existing.Disabled = true

	s.store.WithContext(ctx)
	return s.store.Save(existing, true)
}

// Enable marks a disabled short URL as active again.
func (s *URLService) Enable(ctx context.Context, code string) error {
	if code == "" {
		return fmt.Errorf("code must not be empty")
	}

	if ctx != nil && ctx.Err() != nil {
		return ctx.Err()
	}

	existing, err := s.store.Get(code)
	if err != nil {
		return err
	}

	existing.Disabled = false

	s.store.WithContext(ctx)
	return s.store.Save(existing, true)
}

// CleanupExpired removes all expired short URLs.
func (s *URLService) CleanupExpired(ctx context.Context) (int, error) {
	if ctx != nil && ctx.Err() != nil {
		return 0, ctx.Err()
	}

	snapshot := s.store.RawSnapshot()
	now := time.Now()
	cleaned := 0

	for _, su := range snapshot {
		if su.IsExpired(now) {
			suCopy := su
			s.store.WithContext(ctx)
			if err := s.store.Save(&suCopy, true); err != nil {
				continue
			}
			cleaned++
		}
	}

	return cleaned, nil
}
