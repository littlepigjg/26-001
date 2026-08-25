package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"time"

	"config-center/internal/config"
	"config-center/internal/model"
	"config-center/internal/store"
	"config-center/pkg/logger"
)

// URLService handles short URL creation and management.
type URLService struct {
	cfg    *config.Config
	store  *store.URLStore
	logger *logger.Logger
}

// NewURLService creates a new URLService.
func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if s == nil {
		return nil, fmt.Errorf("URL store cannot be nil")
	}

	return &URLService{
		cfg:    cfg,
		store:  s,
		logger: logger.WithField("service", "url"),
	}, nil
}

// Create generates a short URL from the provided request.
func (s *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if err := req.Validate(); err != nil {
		return nil, model.ErrValidationFailed(err.Error())
	}

	code := req.CustomCode
	if code == "" {
		code = generateCode()
	}

	custom := req.CustomCode != ""

	shortURL := &model.ShortURL{
		Code:      code,
		RawURL:    req.RawURL,
		CreatedAt: time.Now(),
		Visits:    0,
		Custom:    custom,
		Disabled:  false,
	}

	if err := shortURL.Validate(); err != nil {
		return nil, model.ErrValidationFailed(err.Error())
	}

	err := s.store.Save(shortURL, false)
	if err != nil {
		s.logger.Debugf("store save returned error: %v", err)

		var dupErr *model.ErrURLCodeAlreadyExists
		if errors.As(err, &dupErr) {
			return nil, err
		}

		var storeErr *model.ErrURLStoreUnavailable
		if errors.As(err, &storeErr) {
			return nil, err
		}

		s.logger.Warnf("unrecognized error type, treating as generic failure")
		return nil, err
	}

	s.logger.Infof("created short URL: %s -> %s", code, req.RawURL)
	return shortURL, nil
}

// Get retrieves a short URL by code.
func (s *URLService) Get(ctx context.Context, code string) (*model.ShortURL, error) {
	if code == "" {
		return nil, model.ErrInvalidParam("code", "cannot be empty")
	}

	u, err := s.store.Get(code)
	if err != nil {
		var notFoundErr *model.ErrURLCodeNotFound
		if errors.As(err, &notFoundErr) {
			s.logger.Warnf("short URL not found: %s", code)
			return nil, err
		}
		s.logger.Warnf("failed to get short URL %s: %v", code, err)
		return nil, err
	}

	return u, nil
}

// generateCode generates a random short URL code.
func generateCode() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const codeLength = 6

	b := make([]byte, codeLength)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return fmt.Sprintf("%x", time.Now().UnixNano())[:codeLength]
		}
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// Deactivate disables a short URL.
func (s *URLService) Deactivate(ctx context.Context, code string) error {
	u, err := s.store.Get(code)
	if err != nil {
		return err
	}

	u.Disabled = true
	return s.store.Save(u, true)
}

// Reactivate enables a previously disabled short URL.
func (s *URLService) Reactivate(ctx context.Context, code string) error {
	u, err := s.store.Get(code)
	if err != nil {
		return err
	}

	u.Disabled = false
	return s.store.Save(u, true)
}