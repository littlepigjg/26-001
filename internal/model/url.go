package model

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ShortURL represents a shortened URL entry.
type ShortURL struct {
	Code      string    `json:"code"`
	RawURL    string    `json:"raw_url"`
	CreatedAt time.Time `json:"created_at"`
	Visits    int       `json:"visits"`
	Custom    bool      `json:"custom"`
	Disabled  bool      `json:"disabled"`
}

// NewShortURL creates a new ShortURL.
func NewShortURL(code, rawURL string, custom bool) *ShortURL {
	return &ShortURL{
		Code:      code,
		RawURL:    rawURL,
		CreatedAt: time.Now(),
		Visits:    0,
		Custom:    custom,
		Disabled:  false,
	}
}

var (
	validURLRE   = regexp.MustCompile(`^https?://[^\s]+`)
	validCodeRE  = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	maxCodeLen   = 32
	minCodeLen   = 3
	maxURLLen    = 2048
)

// Validate checks the ShortURL for field-level correctness.
func (s *ShortURL) Validate() error {
	if s == nil {
		return fmt.Errorf("short_url is nil")
	}
	if strings.TrimSpace(s.Code) == "" {
		return fmt.Errorf("code is required")
	}
	if len(s.Code) < minCodeLen || len(s.Code) > maxCodeLen {
		return fmt.Errorf("code length must be between %d and %d", minCodeLen, maxCodeLen)
	}
	if !validCodeRE.MatchString(s.Code) {
		return fmt.Errorf("code contains invalid characters")
	}
	if strings.TrimSpace(s.RawURL) == "" {
		return fmt.Errorf("raw_url is required")
	}
	if len(s.RawURL) > maxURLLen {
		return fmt.Errorf("raw_url is too long")
	}
	if !validURLRE.MatchString(s.RawURL) {
		return fmt.Errorf("raw_url must start with http:// or https://")
	}
	if s.Visits < 0 {
		return fmt.Errorf("visits cannot be negative")
	}
	return nil
}

// IsExpired determines whether the ShortURL is expired based on MaxVisits or time window.
func (s *ShortURL) IsExpired(now time.Time) bool {
	if s == nil {
		return true
	}
	if s.Disabled {
		return true
	}
	if s.CreatedAt.IsZero() {
		return true
	}
	if now.Before(s.CreatedAt) {
		return true
	}
	return false
}

// CreateReq is the request payload for creating a new short URL.
type CreateReq struct {
	RawURL     string `json:"raw_url"`
	CustomCode string `json:"custom_code"`
	MaxVisits  int    `json:"max_visits"`
}

// Validate checks the CreateReq for correctness.
func (r *CreateReq) Validate() error {
	if r == nil {
		return fmt.Errorf("create_req is nil")
	}
	if strings.TrimSpace(r.RawURL) == "" {
		return fmt.Errorf("raw_url is required")
	}
	if len(r.RawURL) > maxURLLen {
		return fmt.Errorf("raw_url is too long")
	}
	if !validURLRE.MatchString(r.RawURL) {
		return fmt.Errorf("raw_url must start with http:// or https://")
	}
	if r.CustomCode != "" {
		if len(r.CustomCode) < minCodeLen || len(r.CustomCode) > maxCodeLen {
			return fmt.Errorf("custom_code length must be between %d and %d", minCodeLen, maxCodeLen)
		}
		if !validCodeRE.MatchString(r.CustomCode) {
			return fmt.Errorf("custom_code contains invalid characters")
		}
	}
	if r.MaxVisits < 0 {
		return fmt.Errorf("max_visits cannot be negative")
	}
	return nil
}
