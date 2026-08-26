package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"config-center/internal/config"
	"config-center/internal/model"
	"config-center/internal/store"
)

// URLService handles short URL creation and management.
type URLService struct {
	cfg     *config.Config
	store   *store.URLStore
}

// NewURLService creates a new URLService.
func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	return &URLService{
		cfg:   cfg,
		store: s,
	}, nil
}

// Create generates a new short URL from the given request.
func (s *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if err := req.Validate(); err != nil {
		return nil, model.NewAppError(model.ErrCodeInvalidParam, err.Error())
	}

	var code string
	if req.CustomCode != "" {
		code = req.CustomCode
	} else {
		code = generateCode(6)
	}

	entry := &model.ShortURL{
		Code:      code,
		RawURL:    req.RawURL,
		CreatedAt: time.Now(),
		Visits:    0,
		Custom:    req.CustomCode != "",
		Disabled:  false,
	}

	if err := entry.Validate(); err != nil {
		return nil, model.ErrValidationFailed(err.Error())
	}

	if err := s.store.Save(entry, false); err != nil {
		return nil, fmt.Errorf("failed to save short url: %w", err)
	}

	return entry, nil
}

// Get retrieves a short URL by its code.
func (s *URLService) Get(ctx context.Context, code string) (*model.ShortURL, error) {
	if code == "" {
		return nil, model.ErrInvalidParam("code", "cannot be empty")
	}

	entry, err := s.store.Get(code)
	if err != nil {
		return nil, fmt.Errorf("failed to get short url: %w", err)
	}

	return entry, nil
}

// GenerateCode generates a random hex code of the specified byte length.
func generateCode(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

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

// RedirectService handles URL redirection.
type RedirectService struct {
	urlStore  *store.URLStore
	logStore  *store.AccessLogStore
}

// NewRedirectService creates a new RedirectService.
func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	return &RedirectService{
		urlStore: us,
		logStore: ls,
	}, nil
}

// HandleRedirect processes a redirect request for the given code.
func (s *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	entry, err := s.urlStore.Get(req.Code)
	if err != nil {
		return nil, fmt.Errorf("failed to get short url: %w", err)
	}

	if entry.IsExpired(req.Timestamp) {
		return nil, model.NewAppError(
			model.ErrCodeNotFound,
			fmt.Sprintf("short url with code '%s' has expired", req.Code),
		)
	}

	entry.Visits++

	s.logStore.Append(store.AccessLogEntry{
		Code:      req.Code,
		Timestamp: req.Timestamp,
	})

	return &RedirectResult{
		RawURL: entry.RawURL,
		Status: 302,
	}, nil
}
