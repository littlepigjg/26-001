package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"config-center/internal/config"
	"config-center/internal/model"
	"config-center/internal/store"
	"config-center/pkg/logger"
)

// URLService handles short URL creation and management.
type URLService struct {
	cfg   *config.Config
	store *store.URLStore
	logger *logger.Logger
}

// NewURLService creates a new URLService.
func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if s == nil {
		return nil, fmt.Errorf("URLStore cannot be nil")
	}

	return &URLService{
		cfg:    cfg,
		store:  s,
		logger: logger.WithField("service", "url"),
	}, nil
}

// Create generates a short URL from the given request.
func (s *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if req == nil {
		return nil, fmt.Errorf("create request cannot be nil")
	}

	if err := req.Validate(); err != nil {
		return nil, err
	}

	var code string
	if req.CustomCode != "" {
		code = req.CustomCode
	} else {
		var err error
		code, err = generateCode(6)
		if err != nil {
			return nil, fmt.Errorf("failed to generate code: %w", err)
		}
	}

	shortURL := &model.ShortURL{
		Code:      code,
		RawURL:    req.RawURL,
		CreatedAt: time.Now(),
		Visits:    0,
		Custom:    req.CustomCode != "",
		Disabled:  false,
	}

	if err := s.store.Save(shortURL, false); err != nil {
		s.logger.Errorf("failed to save short URL: %v", err)
		return nil, err
	}

	s.logger.Infof("created short URL: %s -> %s", code, req.RawURL)
	return shortURL, nil
}

// Get retrieves a short URL by its code.
func (s *URLService) Get(ctx context.Context, code string) (*model.ShortURL, error) {
	return s.store.Get(code)
}

// Delete removes a short URL by its code.
func (s *URLService) Delete(ctx context.Context, code string) error {
	u, err := s.store.Get(code)
	if err != nil {
		return err
	}

	u.Disabled = true
	return s.store.Save(u, true)
}

// generateCode generates a random URL code of the specified length.
func generateCode(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b)[:length], nil
}
