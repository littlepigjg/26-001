package service

import (
	"context"
	"fmt"
	"time"

	"config-center/internal/store"
)

type RedirectRequest struct {
	Code      string    `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}

type RedirectResult struct {
	RawURL string `json:"raw_url"`
	Status int    `json:"status"`
}

type RedirectService struct {
	us *store.URLStore
	ls *store.AccessLogStore
}

func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	if us == nil {
		return nil, fmt.Errorf("url store cannot be nil")
	}
	if ls == nil {
		return nil, fmt.Errorf("access log store cannot be nil")
	}
	return &RedirectService{us: us, ls: ls}, nil
}

func (svc *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	u, err := svc.us.Get(req.Code)
	if err != nil {
		return nil, fmt.Errorf("get short URL failed: %w", err)
	}

	if u.Disabled {
		return nil, fmt.Errorf("short URL %s is disabled", req.Code)
	}

	return &RedirectResult{
		RawURL: u.RawURL,
		Status: 302,
	}, nil
}
