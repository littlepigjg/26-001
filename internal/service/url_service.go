package service

import (
	"context"
	"fmt"
	"time"

	"config-center/internal/config"
	"config-center/internal/model"
	"config-center/internal/store"
)

type URLService struct {
	cfg *config.Config
	s   *store.URLStore
}

func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if s == nil {
		return nil, fmt.Errorf("url store cannot be nil")
	}
	return &URLService{cfg: cfg, s: s}, nil
}

func (svc *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	code := req.CustomCode
	if code == "" {
		code = fmt.Sprintf("s-%d", time.Now().UnixNano())
	}

	u := model.NewShortURL(code, req.RawURL, req.CustomCode != "")

	if err := svc.s.Save(u, false); err != nil {
		return nil, fmt.Errorf("save short URL failed: %w", err)
	}

	return u, nil
}
