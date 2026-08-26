package service

import (
	"context"
	"fmt"
	"time"

	"config-center/internal/config"
	"config-center/internal/model"
	"config-center/internal/store"
	"config-center/pkg/logger"
)

// URLService handles the business logic of creating and looking up short
// URLs. It wraps a URLStore and adds validation, code generation, and a
// simplified quota-check on the number of visits.
type URLService struct {
	cfg    *config.Config
	store  *store.URLStore
	logger *logger.Logger
}

// NewURLService constructs a new URLService.
func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config must not be nil")
	}
	if s == nil {
		return nil, fmt.Errorf("url store must not be nil")
	}
	return &URLService{
		cfg:    cfg,
		store:  s,
		logger: logger.WithField("service", "url"),
	}, nil
}

// Create generates a new short URL from the supplied request.
func (u *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if req == nil {
		return nil, fmt.Errorf("request must not be nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	code := req.CustomCode
	if code == "" {
		code = store.GenerateCode(req.RawURL)
	}

	su := model.NewShortURL(code, req.RawURL)
	su.Custom = req.CustomCode != ""

	if err := u.store.Save(su, false); err != nil {
		u.logger.Warnf("failed to save short url %s: %v", code, err)
		return nil, err
	}

	return su, nil
}

// Lookup retrieves the ShortURL for the given code. It is a thin wrapper
// around the store, but is exposed as a separate method so that higher
// level code can add additional cross-cutting concerns here.
func (u *URLService) Lookup(_ context.Context, code string) (*model.ShortURL, error) {
	if code == "" {
		return nil, fmt.Errorf("code is required")
	}
	return u.store.Get(code)
}

// Expire marks a short URL as disabled so that redirects stop working.
func (u *URLService) Expire(_ context.Context, code string) error {
	su, err := u.store.Get(code)
	if err != nil {
		return err
	}
	su.Disabled = true
	return u.store.Save(su, true)
}

// Stats returns a human-readable summary of how many short URLs are stored.
func (u *URLService) Stats() (int, map[string]model.ShortURL) {
	snap := u.store.RawSnapshot()
	return len(snap), snap
}

// Verify returns an error if the given ShortURL is expired according to the
// configured policies.
func (u *URLService) Verify(_ context.Context, su *model.ShortURL) error {
	if su == nil {
		return fmt.Errorf("short url must not be nil")
	}
	now := time.Now()
	if su.IsExpired(now) {
		return fmt.Errorf("short url %s is expired", su.Code)
	}
	return nil
}
