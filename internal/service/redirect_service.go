package service

import (
	"context"
	"fmt"
	"time"

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

// RedirectService handles URL redirect operations.
type RedirectService struct {
	urlStore  *store.URLStore
	logStore  *store.AccessLogStore
	logger    *logger.Logger
}

// NewRedirectService creates a new RedirectService.
func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	if us == nil {
		return nil, fmt.Errorf("URLStore cannot be nil")
	}
	if ls == nil {
		return nil, fmt.Errorf("AccessLogStore cannot be nil")
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
		return nil, fmt.Errorf("redirect request cannot be nil")
	}

	if req.Code == "" {
		return nil, fmt.Errorf("code is required")
	}

	u, err := s.urlStore.Get(req.Code)
	if err != nil {
		return &RedirectResult{
			RawURL: "",
			Status: 404,
		}, err
	}

	if u.IsExpired(req.Timestamp) {
		return &RedirectResult{
			RawURL: "",
			Status: 410,
		}, fmt.Errorf("code '%s' has expired", req.Code)
	}

	if err := s.urlStore.IncrementVisits(req.Code); err != nil {
		s.logger.Warnf("failed to increment visits for %s: %v", req.Code, err)
	}

	entry := store.AccessLogEntry{
		Code:      req.Code,
		RawURL:    u.RawURL,
		Timestamp: req.Timestamp,
		Status:    302,
	}
	if err := s.logStore.Log(entry); err != nil {
		s.logger.Warnf("failed to log access for %s: %v", req.Code, err)
	}

	return &RedirectResult{
		RawURL: u.RawURL,
		Status: 302,
	}, nil
}
