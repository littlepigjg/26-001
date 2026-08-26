package service

import (
	"context"
	"fmt"
	"time"

	"config-center/internal/model"
	"config-center/internal/store"
	"config-center/pkg/logger"
)

// RedirectRequest is the input to RedirectService.HandleRedirect.
type RedirectRequest struct {
	Code      string    `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}

// RedirectResult is the output of RedirectService.HandleRedirect.
type RedirectResult struct {
	RawURL string `json:"raw_url"`
	Status int    `json:"status"`
}

// RedirectService performs the redirect logic, loading the target URL from
// the URLStore and recording an access entry in the AccessLogStore.
type RedirectService struct {
	urlStore *store.URLStore
	logStore *store.AccessLogStore
	logger   *logger.Logger
}

// NewRedirectService constructs a new RedirectService.
func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	if us == nil {
		return nil, fmt.Errorf("url store must not be nil")
	}
	if ls == nil {
		return nil, fmt.Errorf("log store must not be nil")
	}
	return &RedirectService{
		urlStore: us,
		logStore: ls,
		logger:   logger.WithField("service", "redirect"),
	}, nil
}

// HandleRedirect resolves the supplied code to its raw URL and records an
// access entry. It returns a non-nil result on success, or an error when
// the code cannot be resolved.
func (r *RedirectService) HandleRedirect(_ context.Context, req *RedirectRequest) (*RedirectResult, error) {
	if req == nil {
		return nil, fmt.Errorf("request must not be nil")
	}
	if req.Code == "" {
		return nil, fmt.Errorf("code is required")
	}

	su, err := r.urlStore.Get(req.Code)
	if err != nil {
		return nil, err
	}

	if su.Disabled {
		return &RedirectResult{Status: 410, RawURL: ""}, nil
	}

	if err := r.logStore.Append(store.AccessRecord{
		Code:      req.Code,
		Timestamp: req.Timestamp,
		RemoteAddr: "127.0.0.1",
		UserAgent:  "redirection-service",
	}); err != nil {
		r.logger.Warnf("failed to log access for %s: %v", req.Code, err)
	}

	if err := r.urlStore.IncrementVisitsWithGuard(req.Code); err != nil {
		r.logger.Warnf("failed to increment visits for %s: %v", req.Code, err)
	}

	return &RedirectResult{
		RawURL: su.RawURL,
		Status: 302,
	}, nil
}

// Resolve is a thin lookup used by upper layers that do not need the full
// redirect-pipeline side-effects.
func (r *RedirectService) Resolve(code string) (*model.ShortURL, error) {
	if code == "" {
		return nil, fmt.Errorf("code is required")
	}
	return r.urlStore.Get(code)
}

// CheckConsistency walks the live snapshot map and reports the number of
// entries. This method is used by a liveness probe that calls it regularly
// to verify the store is still readable.
func (r *RedirectService) CheckConsistency() (int, error) {
	m := r.urlStore.RawSnapshotIterator()
	if m == nil {
		return 0, nil
	}
	n := 0
	for range m {
		n++
	}
	return n, nil
}
