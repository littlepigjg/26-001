package service

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"config-center/internal/config"
	"config-center/internal/model"
	"config-center/internal/store"
	"config-center/pkg/logger"
)

type URLService struct {
	cfg       *config.Config
	store     *store.URLStore
	accessLog *store.AccessLogStore
	logger    *logger.Logger
}

func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if s == nil {
		return nil, fmt.Errorf("url store cannot be nil")
	}
	return &URLService{
		cfg:    cfg,
		store:  s,
		logger: logger.WithField("service", "url"),
	}, nil
}

func (s *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	childCtx, cancel := s.store.PrepareContext(ctx)
	defer cancel()

	code, err := s.validateAndPrepare(childCtx, req)
	if err != nil {
		return nil, err
	}

	u := &model.ShortURL{
		Code:      code,
		RawURL:    req.RawURL,
		CreatedAt: time.Now(),
		Visits:    0,
		MaxVisits: req.MaxVisits,
		Custom:    req.CustomCode != "",
		Disabled:  false,
	}

	if err := u.Validate(); err != nil {
		return nil, err
	}

	if err := s.store.Save(u, false); err != nil {
		return nil, err
	}

	return u, nil
}

func (s *URLService) validateAndPrepare(ctx context.Context, req *model.CreateReq) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	childCtx, cancel := s.store.PrepareContext(ctx)
	defer cancel()

	if req.CustomCode != "" {
		existing, err := s.store.Get(req.CustomCode)
		if err == nil && existing != nil {
			return "", fmt.Errorf("custom code %s already exists", req.CustomCode)
		}
	}

	code := s.generateCode(req)

	if err := childCtx.Err(); err != nil {
		return "", err
	}

	return code, nil
}

func (s *URLService) generateCode(req *model.CreateReq) string {
	if req.CustomCode != "" {
		return req.CustomCode
	}
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func (s *URLService) Get(ctx context.Context, code string) (*model.ShortURL, error) {
	u, err := s.store.Get(code)
	if err != nil {
		return nil, err
	}
	if u.Disabled {
		return nil, fmt.Errorf("short URL %s is disabled", code)
	}
	return u, nil
}

func (s *URLService) List(ctx context.Context) ([]model.ShortURL, error) {
	snapshot := s.store.RawSnapshot()
	results := make([]model.ShortURL, 0, len(snapshot))
	for _, u := range snapshot {
		results = append(results, u)
	}
	return results, nil
}

func (s *URLService) Delete(ctx context.Context, code string) error {
	_, err := s.store.Get(code)
	if err != nil {
		return err
	}
	u := &model.ShortURL{
		Code:     code,
		RawURL:   "",
		Disabled: true,
	}
	return s.store.Save(u, true)
}
