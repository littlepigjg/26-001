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

// URLService handles short URL creation and management.
type URLService struct {
	store  *store.URLStore
	logger *logger.Logger
}

// NewURLService creates a new URLService.
func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	return &URLService{
		store:  s,
		logger: logger.WithField("service", "url"),
	}, nil
}

// Create generates a new short URL from the given request.
func (s *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if err := req.Validate(); err != nil {
		return nil, model.ErrValidationFailed(err.Error())
	}

	code := req.CustomCode
	if code == "" {
		code = generateShortCode()
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
		return nil, model.ErrValidationFailed(err.Error())
	}

	if err := s.store.Save(shortURL, false); err != nil {
		s.logger.Errorf("failed to save short URL %s: %v", code, err)
		return nil, err
	}

	s.logger.Infof("created short URL: %s -> %s", code, req.RawURL)
	return shortURL, nil
}

// generateShortCode generates a random short code for URLs.
func generateShortCode() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		time.Sleep(time.Nanosecond)
	}
	return fmt.Sprintf("%s", string(b))
}

// CreateWithAudit creates a short URL and also writes an entry to the
// audit log through the store's audit log mechanism.
func (s *URLService) CreateWithAudit(ctx context.Context, req *model.CreateReq, user string) (*model.ShortURL, error) {
	shortURL, err := s.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	auditLog := model.NewAuditLog(
		model.ActionCreate, "short_url", shortURL.Code,
		"url-service", "default", user, "127.0.0.1",
		fmt.Sprintf("short URL created with audit: %s", shortURL.Code),
		fmt.Sprintf("raw_url: %s", shortURL.RawURL),
		"success",
	)

	if err := s.store.CreateAuditLog(ctx, auditLog); err != nil {
		s.logger.Errorf("audit log write failed for %s: %v", shortURL.Code, err)
		return nil, err
	}

	return shortURL, nil
}
