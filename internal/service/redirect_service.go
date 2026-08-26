package service

import (
	"context"
	"fmt"
	"time"

	"config-center/internal/model"
	"config-center/internal/store"
	"config-center/pkg/logger"
)

// RedirectService handles short URL redirects and access logging.
type RedirectService struct {
	urlStore *store.URLStore
	logStore *store.AccessLogStore
	logger   *logger.Logger
}

// NewRedirectService creates a new RedirectService.
func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	return &RedirectService{
		urlStore: us,
		logStore: ls,
		logger:   logger.WithField("service", "redirect"),
	}, nil
}

// HandleRedirect processes a redirect request for a given short code.
// It retrieves the original URL, increments the visit counter, and
// writes an access log entry.
func (s *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	shortURL, err := s.urlStore.Get(req.Code)
	if err != nil {
		return nil, err
	}

	if shortURL.Disabled {
		return &RedirectResult{
			RawURL: "",
			Status: 410,
		}, nil
	}

	if shortURL.IsExpired(req.Timestamp) {
		return &RedirectResult{
			RawURL: "",
			Status: 410,
		}, nil
	}

	if err := s.urlStore.IncrementVisitsWithGuard(req.Code); err != nil {
		s.logger.Warnf("failed to increment visits for %s: %v", req.Code, err)
	}

	accessLog := model.NewAuditLog(
		model.ActionLogin, "short_url", req.Code,
		"redirect-service", "default", "anonymous", "127.0.0.1",
		fmt.Sprintf("redirect: %s -> %s", req.Code, shortURL.RawURL),
		"",
		"success",
	)

	if err := s.logStore.Write(accessLog); err != nil {
		s.logger.Errorf("failed to write access log for %s: %v", req.Code, err)
	}

	return &RedirectResult{
		RawURL: shortURL.RawURL,
		Status: 302,
	}, nil
}

// RedirectRequest represents a redirect request.
type RedirectRequest struct {
	Code      string
	Timestamp time.Time
}

// RedirectResult represents a redirect response.
type RedirectResult struct {
	RawURL string
	Status int
}
