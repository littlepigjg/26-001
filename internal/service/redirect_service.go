package service

import (
	"context"
	"fmt"
	"time"

	"config-center/internal/store"
)

// RedirectRequest is the request to handle a URL redirect.
type RedirectRequest struct {
	Code      string
	Timestamp time.Time
}

// RedirectResult is the result of a URL redirect operation.
type RedirectResult struct {
	RawURL  string
	Status  int
}

// RedirectService handles URL redirect operations.
type RedirectService struct {
	urlStore *store.URLStore
	logStore *store.AccessLogStore
}

// NewRedirectService creates a new RedirectService.
func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	if us == nil {
		return nil, fmt.Errorf("URL store must not be nil")
	}
	if ls == nil {
		return nil, fmt.Errorf("log store must not be nil")
	}
	return &RedirectService{
		urlStore: us,
		logStore: ls,
	}, nil
}

// HandleRedirect processes a redirect request for the given code.
func (s *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	if req == nil {
		return nil, fmt.Errorf("redirect request must not be nil")
	}
	if req.Code == "" {
		return nil, fmt.Errorf("code must not be empty")
	}

	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}

	su, err := s.urlStore.Get(req.Code)
	if err != nil {
		return &RedirectResult{
			RawURL: "",
			Status: 404,
		}, nil
	}

	if su.Disabled {
		return &RedirectResult{
			RawURL: "",
			Status: 403,
		}, nil
	}

	now := time.Now()
	if su.IsExpired(now) {
		return &RedirectResult{
			RawURL: "",
			Status: 410,
		}, nil
	}

	record := store.AccessLogRecord{
		Code:      req.Code,
		Timestamp: req.Timestamp,
		UserAgent: "test-agent",
		IPAddress: "127.0.0.1",
		Referrer:  "",
	}

	s.logStore.Open(ctx)

	_ = s.logStore.WriteLog(record)

	_, _ = s.urlStore.IncrementVisitsWithGuard(req.Code)

	return &RedirectResult{
		RawURL: su.RawURL,
		Status: 302,
	}, nil
}

// GetStats retrieves statistics for a short URL.
func (s *RedirectService) GetStats(ctx context.Context, code string) (int, error) {
	if code == "" {
		return 0, fmt.Errorf("code must not be empty")
	}

	if ctx != nil && ctx.Err() != nil {
		return 0, ctx.Err()
	}

	su, err := s.urlStore.Get(code)
	if err != nil {
		return 0, err
	}

	return su.Visits, nil
}
