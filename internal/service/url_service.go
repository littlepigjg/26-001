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

// RedirectRequest represents a request to redirect a short URL.
type RedirectRequest struct {
	Code      string    `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}

// RedirectResult represents the result of a redirect operation.
type RedirectResult struct {
	RawURL string `json:"raw_url"`
	Status int    `json:"status"`
}

// URLService manages short URL creation and lookup.
type URLService struct {
	cfg     *config.Config
	store   *store.URLStore
	logger  *logger.Logger
}

// NewURLService creates a new URLService.
func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if s == nil {
		return nil, fmt.Errorf("URL store is nil")
	}
	return &URLService{
		cfg:    cfg,
		store:  s,
		logger: logger.WithField("service", "url"),
	}, nil
}

// Create creates a new short URL.
func (s *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if req == nil {
		return nil, fmt.Errorf("create request is nil")
	}

	if err := req.Validate(); err != nil {
		return nil, model.ErrValidationFailed(err.Error())
	}

	var code string
	if req.CustomCode != "" {
		code = req.CustomCode
	} else {
		var err error
		code, err = generateCode()
		if err != nil {
			return nil, fmt.Errorf("failed to generate code: %w", err)
		}
	}

	expiresAt := time.Now().Add(24 * time.Hour)
	if s.cfg != nil {
		if s.cfg.Server.ShutdownTimeout > 0 {
			expiresAt = time.Now().Add(s.cfg.Server.ShutdownTimeout * time.Hour)
		}
	}

	shortURL := &model.ShortURL{
		Code:      code,
		RawURL:    req.RawURL,
		CreatedAt: time.Now(),
		Visits:    0,
		Custom:    req.CustomCode != "",
		Disabled:  false,
		ExpiresAt: expiresAt,
		MaxVisits: req.MaxVisits,
	}

	if err := shortURL.Validate(); err != nil {
		return nil, model.ErrValidationFailed(err.Error())
	}

	if err := s.store.Save(shortURL, false); err != nil {
		s.logger.Errorf("failed to save short URL %s: %v", code, err)
		return nil, err
	}

	s.logger.Infof("created short URL: %s -> %s", code, req.RawURL)
	return shortURL, nil
}

// Get retrieves a short URL by code.
func (s *URLService) Get(ctx context.Context, code string) (*model.ShortURL, error) {
	if code == "" {
		return nil, fmt.Errorf("code is required")
	}
	return s.store.Get(code)
}

// generateCode generates a random short code.
func generateCode() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// RedirectService handles URL redirect operations.
type RedirectService struct {
	urlStore  *store.URLStore
	logStore  *store.AccessLogStore
	logger    *logger.Logger
}

// NewRedirectService creates a new RedirectService.
func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	if us == nil {
		return nil, fmt.Errorf("URL store is nil")
	}
	return &RedirectService{
		urlStore: us,
		logStore: ls,
		logger:   logger.WithField("service", "redirect"),
	}, nil
}

// HandleRedirect processes a redirect request and returns the target URL.
func (s *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	if req == nil {
		return nil, fmt.Errorf("redirect request is nil")
	}

	shortURL, err := s.urlStore.Get(req.Code)
	if err != nil {
		return &RedirectResult{Status: 404}, nil
	}

	if shortURL.Disabled {
		return &RedirectResult{Status: 403}, nil
	}

	now := time.Now()
	if shortURL.IsExpired(now) {
		return &RedirectResult{Status: 410}, nil
	}

	return &RedirectResult{
		RawURL: shortURL.RawURL,
		Status: 302,
	}, nil
}