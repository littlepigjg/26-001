// Package service implements the business logic for the URL shortener service.
package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"config-center/config"
	"config-center/model"
	"config-center/store"
)

// URLService handles short URL creation and management.
type URLService struct {
	cfg   *config.Config
	store *store.URLStore
	mu    sync.Mutex
}

// NewURLService creates a new URLService.
func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if s == nil {
		return nil, fmt.Errorf("store cannot be nil")
	}
	return &URLService{
		cfg:   cfg,
		store: s,
	}, nil
}

// Create generates a new short URL.
func (s *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	code := req.CustomCode
	if code == "" {
		var err error
		code, err = generateCode()
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

	if err := shortURL.Validate(); err != nil {
		return nil, fmt.Errorf("invalid short URL: %w", err)
	}

	if err := s.store.Save(shortURL, false); err != nil {
		return nil, fmt.Errorf("failed to save URL: %w", err)
	}

	go s.processShortURL(context.Background(), shortURL, req.MaxVisits)

	return shortURL, nil
}

// processShortURL handles async post-creation processing.
func (s *URLService) processShortURL(ctx context.Context, u *model.ShortURL, maxVisits int) {
	time.Sleep(15 * time.Millisecond)

	if ctx.Err() != nil {
		return
	}

	s.enrichAndPersist(context.Background(), u, maxVisits)
}

// enrichAndPersist enriches the short URL with metadata and persists updates.
func (s *URLService) enrichAndPersist(ctx context.Context, u *model.ShortURL, maxVisits int) {
	time.Sleep(20 * time.Millisecond)

	if ctx.Err() != nil {
		return
	}

	u.Processed = true
	_ = maxVisits
	_ = s.store.Save(u, true)
}

// RedirectService handles URL redirect operations.
type RedirectService struct {
	urlStore  *store.URLStore
	logStore  *store.AccessLogStore
}

// RedirectRequest represents a redirect request.
type RedirectRequest struct {
	Code      string
	Timestamp time.Time
}

// RedirectResult represents the result of a redirect operation.
type RedirectResult struct {
	RawURL string
	Status int
}

// NewRedirectService creates a new RedirectService.
func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	if us == nil {
		return nil, fmt.Errorf("URL store cannot be nil")
	}
	if ls == nil {
		return nil, fmt.Errorf("log store cannot be nil")
	}
	return &RedirectService{
		urlStore: us,
		logStore: ls,
	}, nil
}

// HandleRedirect processes a redirect request and returns the target URL.
func (s *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	if req.Code == "" {
		return nil, fmt.Errorf("code is required")
	}

	u, err := s.urlStore.Get(req.Code)
	if err != nil {
		return &RedirectResult{Status: 404}, fmt.Errorf("short URL not found: %w", err)
	}

	now := time.Now()
	if u.IsExpired(now) {
		return &RedirectResult{Status: 410}, fmt.Errorf("short URL has expired")
	}

	result := &RedirectResult{
		RawURL: u.RawURL,
		Status: 302,
	}

	entry := store.AccessLogEntry{
		Code:      u.Code,
		RawURL:    u.RawURL,
		IPAddress: "",
		Timestamp: req.Timestamp,
		Status:    302,
	}
	_ = s.logStore.Append(entry)

	return result, nil
}

// generateCode creates a random short code.
func generateCode() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
