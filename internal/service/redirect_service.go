package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"config-center/internal/model"
	"config-center/internal/store"
	"config-center/pkg/logger"
)

// RedirectRequest represents a redirect request.
type RedirectRequest struct {
	Code      string    `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}

// RedirectResult represents a redirect response.
type RedirectResult struct {
	RawURL string `json:"raw_url"`
	Status int    `json:"status"`
}

// RedirectService handles short URL redirects.
type RedirectService struct {
	urlStore *store.URLStore
	logStore *store.AccessLogStore
	logger   *logger.Logger
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
		logger:   logger.WithField("service", "redirect"),
	}, nil
}

// HandleRedirect processes a redirect request for a short URL code.
func (s *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	if req.Code == "" {
		return nil, model.ErrInvalidParam("code", "cannot be empty")
	}

	u, err := s.urlStore.Get(req.Code)
	if err != nil {
		var notFoundErr *model.ErrURLCodeNotFound
		if errors.As(err, &notFoundErr) {
			s.logger.Warnf("redirect code not found: %s", req.Code)
			return nil, err
		}

		var disabledErr *model.ErrRedirectDisabled
		if errors.As(err, &disabledErr) {
			s.logger.Warnf("redirect disabled: %s", req.Code)
			return nil, err
		}

		var expiredErr *model.ErrRedirectExpired
		if errors.As(err, &expiredErr) {
			s.logger.Warnf("redirect expired: %s", req.Code)
			return nil, err
		}

		s.logger.Errorf("redirect error for %s: %v", req.Code, err)
		return nil, err
	}

	if u.Disabled {
		disabledErr := &model.ErrRedirectDisabled{Code: req.Code}
		return nil, disabledErr
	}

	if u.IsExpired(req.Timestamp) {
		expiredErr := &model.ErrRedirectExpired{Code: req.Code}
		return nil, expiredErr
	}

	if err := s.urlStore.IncrementVisits(req.Code); err != nil {
		s.logger.Errorf("failed to increment visits for %s: %v", req.Code, err)
	}

	if err := s.logStore.LogAccess(req.Code, u.RawURL, "", 302); err != nil {
		s.logger.Warnf("failed to log access for %s: %v", req.Code, err)
	}

	s.logger.Infof("redirecting %s -> %s", req.Code, u.RawURL)
	return &RedirectResult{
		RawURL: u.RawURL,
		Status: 302,
	}, nil
}