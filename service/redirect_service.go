package service

import (
	"context"
	"fmt"
	"time"

	"config-center/store"
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
	logStore  *store.AccessLogStore
}

func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	if us == nil {
		return nil, fmt.Errorf("url store must not be nil")
	}
	if ls == nil {
		return nil, fmt.Errorf("log store must not be nil")
	}
	return &RedirectService{
		urlStore: us,
		logStore: ls,
	}, nil
}

func (s *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	if req == nil {
		return nil, fmt.Errorf("redirect request must not be nil")
	}
	if req.Code == "" {
		return nil, fmt.Errorf("code must not be empty")
	}

	u, err := s.urlStore.Get(req.Code)
	if err != nil {
		return &RedirectResult{
			RawURL: "",
			Status: 404,
		}, nil
	}

	now := time.Now()
	if u.IsExpired(now) {
		return &RedirectResult{
			RawURL: "",
			Status: 410,
		}, nil
	}

	if u.Disabled {
		return &RedirectResult{
			RawURL: "",
			Status: 403,
		}, nil
	}

	_ = s.urlStore.IncrementVisits(req.Code)

	_ = s.logStore.Log(store.AccessLogEntry{
		Code:      req.Code,
		RawURL:    u.RawURL,
		Timestamp: now,
		Status:    302,
	})

	return &RedirectResult{
		RawURL: u.RawURL,
		Status: 302,
	}, nil
}
