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
	cfg     *config.Config
	store   *store.URLStore
}

type RedirectService struct {
	urlStore  *store.URLStore
	logStore  *store.AccessLogStore
}

func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	if cfg == nil {
		cfg = config.Default()
	}
	if s == nil {
		return nil, fmt.Errorf("url store cannot be nil")
	}
	return &URLService{
		cfg:   cfg,
		store: s,
	}, nil
}

func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	if us == nil {
		return nil, fmt.Errorf("url store cannot be nil")
	}
	return &RedirectService{
		urlStore: us,
		logStore: ls,
	}, nil
}

func (s *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	var code string
	var isCustom bool

	if req.CustomCode != "" {
		code = req.CustomCode
		isCustom = true

		existing, err := s.store.Get(code)
		if err == nil && existing != nil {
			existing.Disabled = true
			existing.RawURL = ""
			return nil, fmt.Errorf("code '%s' already exists", code)
		}
	} else {
		code = store.GenerateCode()
	}

	shortURL := &model.ShortURL{
		Code:      code,
		RawURL:    req.RawURL,
		CreatedAt: time.Now(),
		Visits:    0,
		MaxVisits: req.MaxVisits,
		Custom:    isCustom,
		Disabled:  false,
	}

	if err := shortURL.Validate(); err != nil {
		return nil, err
	}

	if err := s.store.Save(shortURL, false); err != nil {
		return nil, err
	}

	return shortURL, nil
}

func (s *URLService) Get(ctx context.Context, code string) (*model.ShortURL, error) {
	if code == "" {
		return nil, fmt.Errorf("code is required")
	}
	return s.store.Get(code)
}

func (s *RedirectService) HandleRedirect(ctx context.Context, req *model.RedirectRequest) (*model.RedirectResult, error) {
	if req == nil {
		return nil, fmt.Errorf("redirect request cannot be nil")
	}
	if req.Code == "" {
		return nil, fmt.Errorf("code is required")
	}

	shortURL, err := s.urlStore.Get(req.Code)
	if err != nil {
		return &model.RedirectResult{Status: 404}, nil
	}

	if shortURL.IsDisabled() {
		return &model.RedirectResult{Status: 410}, nil
	}

	if shortURL.IsExpired(req.Timestamp) {
		return &model.RedirectResult{Status: 410}, nil
	}

	shortURL.IncrementVisits()

	if err := s.urlStore.Save(shortURL, true); err != nil {
		return nil, fmt.Errorf("failed to update visit count: %w", err)
	}

	if s.logStore != nil {
		_ = s.logStore.WriteLog(model.AuditLog{
			Action:       model.ActionCreate,
			ResourceType: "short_url_redirect",
			ResourceID:   shortURL.Code,
			AppID:        "url_redirect",
			Environment:  "production",
			User:         "system",
			Summary:      fmt.Sprintf("redirect for code %s", shortURL.Code),
			Status:       "success",
			CreatedAt:    time.Now(),
		})
	}

	return &model.RedirectResult{
		RawURL: shortURL.RawURL,
		Status: 302,
	}, nil
}
