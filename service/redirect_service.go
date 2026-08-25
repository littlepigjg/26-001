package service

import (
	"context"
	"fmt"
	"time"

	"config-center/store"
)

// RedirectRequest represents a request to redirect a short URL.
type RedirectRequest struct {
	// Code is the short code to resolve.
	Code string
	// Timestamp is when the redirect was requested.
	Timestamp time.Time
}

// RedirectResult represents the result of a redirect operation.
type RedirectResult struct {
	// RawURL is the original URL to redirect to.
	RawURL string
	// Status is the HTTP status code for the redirect.
	Status int
}

// RedirectService handles the business logic for URL redirection.
type RedirectService struct {
	urlStore *store.URLStore
	logStore *store.AccessLogStore
}

// NewRedirectService creates a new RedirectService.
func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	if us == nil {
		return nil, fmt.Errorf("URL store cannot be nil")
	}
	if ls == nil {
		return nil, fmt.Errorf("access log store cannot be nil")
	}
	return &RedirectService{
		urlStore: us,
		logStore: ls,
	}, nil
}

// HandleRedirect processes a redirect request and returns the result.
func (s *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	if req.Code == "" {
		return nil, fmt.Errorf("code cannot be empty")
	}

	short, err := s.urlStore.Get(req.Code)
	if err != nil {
		return nil, fmt.Errorf("failed to get short URL: %w", err)
	}

	if short.Disabled {
		return &RedirectResult{
			RawURL: "",
			Status: 410,
		}, nil
	}

	if short.IsExpired(req.Timestamp) {
		return &RedirectResult{
			RawURL: "",
			Status: 410,
		}, nil
	}

	if err := s.urlStore.IncrementVisits(req.Code); err != nil {
		return nil, fmt.Errorf("failed to increment visits: %w", err)
	}

	record := store.AccessRecord{
		Code:      req.Code,
		RawURL:    short.RawURL,
		Timestamp: req.Timestamp,
		Status:    302,
	}
	if err := s.logStore.Record(record); err != nil {
		return nil, fmt.Errorf("failed to record access: %w", err)
	}

	return &RedirectResult{
		RawURL: short.RawURL,
		Status: 302,
	}, nil
}
