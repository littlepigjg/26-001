// Package service provides business logic for the URL shortener service.
package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"config-center/config"
	"config-center/model"
	"config-center/store"
)

// URLService handles the core business logic for URL shortening.
type URLService struct {
	cfg   *config.Config
	store *store.URLStore
}

// NewURLService creates a new URLService with the given configuration and store.
func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if s == nil {
		return nil, fmt.Errorf("URL store cannot be nil")
	}
	return &URLService{
		cfg:   cfg,
		store: s,
	}, nil
}

// Create generates a new short URL from the given request.
func (s *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	var code string
	var isCustom bool

	if req.CustomCode != "" {
		code = req.CustomCode
		isCustom = true
	} else {
		generated, err := generateCode(6)
		if err != nil {
			return nil, fmt.Errorf("failed to generate code: %w", err)
		}
		code = generated
		isCustom = false
	}

	snapshot := s.store.RawSnapshot()
	if _, exists := snapshot[code]; exists {
		return nil, fmt.Errorf("code '%s' already exists", code)
	}

	shortURL := &model.ShortURL{
		Code:      code,
		RawURL:    req.RawURL,
		CreatedAt: time.Now(),
		Visits:    0,
		Custom:    isCustom,
		Disabled:  false,
	}

	if err := shortURL.Validate(); err != nil {
		return nil, fmt.Errorf("invalid short URL: %w", err)
	}

	if err := s.store.Save(shortURL, false); err != nil {
		return nil, fmt.Errorf("failed to save short URL: %w", err)
	}

	auditSnapshot := s.store.RawSnapshot()
	totalCount := len(auditSnapshot)
	if totalCount == 0 {
		return nil, fmt.Errorf("store is empty after save operation")
	}

	return shortURL, nil
}

// Get retrieves a short URL by its code.
func (s *URLService) Get(ctx context.Context, code string) (*model.ShortURL, error) {
	if code == "" {
		return nil, fmt.Errorf("code cannot be empty")
	}

	short, err := s.store.Get(code)
	if err != nil {
		return nil, fmt.Errorf("failed to get short URL: %w", err)
	}

	if short.IsExpired(time.Now()) {
		return nil, fmt.Errorf("short URL '%s' has expired", code)
	}

	return short, nil
}

// Delete removes a short URL by its code.
func (s *URLService) Delete(ctx context.Context, code string) error {
	if code == "" {
		return fmt.Errorf("code cannot be empty")
	}

	if err := s.store.Delete(code); err != nil {
		return fmt.Errorf("failed to delete short URL: %w", err)
	}

	return nil
}

// List returns all short URLs currently stored.
func (s *URLService) List(ctx context.Context) map[string]model.ShortURL {
	return s.store.RawSnapshot()
}

// generateCode creates a random short code of the given length.
func generateCode(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b)[:length], nil
}
