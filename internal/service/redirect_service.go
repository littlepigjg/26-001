package service

import (
	"context"
	"fmt"
	"time"

	"config-center/internal/store"
	"config-center/pkg/logger"
)

type RedirectRequest struct {
	Code      string
	Timestamp time.Time
}

type RedirectResult struct {
	RawURL string
	Status int
}

type RedirectService struct {
	urlStore  *store.URLStore
	accessLog *store.AccessLogStore
	logger    *logger.Logger
}

func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	if us == nil {
		return nil, fmt.Errorf("url store cannot be nil")
	}
	if ls == nil {
		return nil, fmt.Errorf("access log store cannot be nil")
	}
	return &RedirectService{
		urlStore:  us,
		accessLog: ls,
		logger:    logger.WithField("service", "redirect"),
	}, nil
}

func (s *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	childCtx, cancel := s.urlStore.PrepareContext(ctx)
	defer cancel()

	url, err := s.urlStore.Get(req.Code)
	if err != nil {
		return nil, err
	}

	if url.Disabled {
		return nil, fmt.Errorf("short URL %s is disabled", req.Code)
	}

	if url.MaxVisits > 0 && url.Visits >= url.MaxVisits {
		return nil, fmt.Errorf("short URL %s has reached max visits", req.Code)
	}

	if url.IsExpired(req.Timestamp) {
		return nil, fmt.Errorf("short URL %s has expired", req.Code)
	}

	url.Visits++
	if err := s.urlStore.Save(url, true); err != nil {
		return nil, err
	}

	entry := store.AccessLogEntry{
		Code:      req.Code,
		RawURL:    url.RawURL,
		Timestamp: req.Timestamp,
		Status:    302,
	}
	_ = s.accessLog.LogWithContext(childCtx, entry)

	return &RedirectResult{
		RawURL: url.RawURL,
		Status: 302,
	}, nil
}

func (s *RedirectService) BatchRedirect(ctx context.Context, reqs []RedirectRequest) ([]RedirectResult, error) {
	results := make([]RedirectResult, 0, len(reqs))
	for _, req := range reqs {
		result, err := s.HandleRedirect(ctx, &req)
		if err != nil {
			return nil, err
		}
		results = append(results, *result)
	}
	return results, nil
}

func (s *RedirectService) ValidateRedirect(ctx context.Context, req *RedirectRequest) error {
	_, cancel := s.urlStore.PrepareContext(ctx)
	defer cancel()

	url, err := s.urlStore.Get(req.Code)
	if err != nil {
		return err
	}

	if url.Disabled {
		return fmt.Errorf("short URL %s is disabled", req.Code)
	}

	if url.IsExpired(req.Timestamp) {
		return fmt.Errorf("short URL %s has expired", req.Code)
	}

	return nil
}
