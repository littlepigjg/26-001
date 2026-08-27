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

	// Launch the background processing unit bound to the caller's ctx so that
	// a cancellation signalled after Create returns propagates into the unit and
	// interrupts it. Passing context.Background() here would detach the unit from
	// the caller's lifecycle and leave the cancel signal unobserved.
	go s.processShortURL(ctx, shortURL, req.MaxVisits)

	return shortURL, nil
}

// processShortURL handles async post-creation processing.
//
// The unit observes the ctx it is launched with: every blocking wait is driven
// by a select on ctx.Done() so the cancel signal interrupts the wait instead of
// running to completion, and the same ctx is forwarded into enrichAndPersist
// so the detach point cannot swallow a cancellation that already arrived.
func (s *URLService) processShortURL(ctx context.Context, u *model.ShortURL, maxVisits int) {
	if !sleepOrCancel(ctx, 15*time.Millisecond) {
		return
	}

	s.enrichAndPersist(ctx, u, maxVisits)
}

// enrichAndPersist enriches the short URL with metadata and persists updates.
//
// As with processShortURL, the wait is selectable on ctx.Done() and the
// cancel-checked exit condition runs against the propagated ctx (not a fresh
// context.Background()), so a cancellation signalled before the persist step
// stops the unit before it marks the status field as completed.
func (s *URLService) enrichAndPersist(ctx context.Context, u *model.ShortURL, maxVisits int) {
	if !sleepOrCancel(ctx, 20*time.Millisecond) {
		return
	}

	if ctx.Err() != nil {
		return
	}

	u.Processed = true
	_ = maxVisits
	_ = s.store.Save(u, true)
}

// sleepOrCancel waits for d, returning true when the delay elapses, or false
// when ctx is cancelled before the delay completes. Unlike time.Sleep, the
// wait is interruptible by a cancel signal.
func sleepOrCancel(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
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
