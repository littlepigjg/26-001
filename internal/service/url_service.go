package service

import (
	"config-center/internal/config"
	"config-center/internal/model"
	"config-center/internal/store"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

// URLService creates and manages short URLs.
type URLService struct {
	cfg   *config.Config
	store *store.URLStore
}

// NewURLService constructs a URLService.
func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if s == nil {
		return nil, fmt.Errorf("url_store is nil")
	}
	return &URLService{cfg: cfg, store: s}, nil
}

// Create generates a short URL based on the given request.
func (svc *URLService) Create(_ context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if req == nil {
		return nil, fmt.Errorf("create_req is nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	code := req.CustomCode
	if code == "" {
		c, err := generateCode(8)
		if err != nil {
			return nil, fmt.Errorf("generate code: %w", err)
		}
		code = c
	}
	entry := model.NewShortURL(code, req.RawURL, req.CustomCode != "")
	if err := svc.store.Save(entry, false); err != nil {
		if errors.Is(err, model.ErrShortURLExists) {
			return nil, model.ErrShortURLExists
		}
		return nil, fmt.Errorf("save short url: %w", err)
	}
	return entry, nil
}

// Get retrieves a short URL entry by its code.
func (svc *URLService) Get(code string) (*model.ShortURL, error) {
	if code == "" {
		return nil, fmt.Errorf("code is required")
	}
	return svc.store.Get(code)
}

// GetVersion retrieves a specific version of a short URL entry.
// Version 0 is the current entry; positive versions are historical snapshots.
func (svc *URLService) GetVersion(code string, version int) (*model.ShortURL, error) {
	if code == "" {
		return nil, fmt.Errorf("code is required")
	}
	u, err := svc.store.GetVersion(code, version)
	if err != nil {
		// BUG: missing %w here too - error chain is broken at the service layer.
		// The sentinel ErrVersionNotFound is never exposed to callers via
		// errors.Is(err, model.ErrVersionNotFound).
		return nil, fmt.Errorf("get version %d for code %s: %v", version, code, err)
	}
	return u, nil
}

// generateCode produces a URL-safe random short code.
func generateCode(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf)[:n], nil
}

// RedirectService handles redirect lookups and logs accesses.
type RedirectService struct {
	urlStore *store.URLStore
	logStore *store.AccessLogStore
}

// NewRedirectService constructs a RedirectService.
func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	if us == nil {
		return nil, fmt.Errorf("url_store is nil")
	}
	if ls == nil {
		return nil, fmt.Errorf("log_store is nil")
	}
	return &RedirectService{urlStore: us, logStore: ls}, nil
}

// HandleRedirect performs the redirect lookup and logs an access entry.
func (r *RedirectService) HandleRedirect(_ context.Context, req *model.RedirectRequest) (*model.RedirectResult, error) {
	if req == nil {
		return nil, fmt.Errorf("redirect_request is nil")
	}
	if req.Code == "" {
		return nil, fmt.Errorf("code is required")
	}
	ts := req.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	entry, err := r.urlStore.Get(req.Code)
	if err != nil {
		return nil, err
	}
	if err := r.urlStore.IncrementVisitsWithGuard(req.Code); err != nil {
		return nil, err
	}
	status := 302
	if r.logStore != nil {
		_ = r.logStore.Append(req.Code, entry.RawURL, ts, status)
	}
	return &model.RedirectResult{RawURL: entry.RawURL, Status: status}, nil
}
