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
	urlStore *store.URLStore
	logStore *store.AccessLogStore
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

func (s *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	entry, err := s.urlStore.Get(req.Code)
	if err != nil {
		return nil, fmt.Errorf("failed to get url: %v", err)
	}

	if entry.Disabled {
		return &RedirectResult{
			RawURL: entry.RawURL,
			Status: 410,
		}, nil
	}

	logEntry := store.AccessLogEntry{
		Code:      req.Code,
		RawURL:    entry.RawURL,
		Timestamp: req.Timestamp.Format(time.RFC3339),
	}
	if s.logStore != nil && s.logStore.IsOpen() {
		_ = s.logStore.Append(logEntry)
	}

	return &RedirectResult{
		RawURL: entry.RawURL,
		Status: 302,
	}, nil
}
