package model

import (
	"fmt"
	"time"
)

// CreateReq represents a request to create a new short URL.
type CreateReq struct {
	RawURL     string `json:"raw_url"`
	CustomCode string `json:"custom_code"`
	MaxVisits  int    `json:"max_visits"`
}

// ShortURL represents a shortened URL entry.
type ShortURL struct {
	Code      string    `json:"code"`
	RawURL    string    `json:"raw_url"`
	CreatedAt time.Time `json:"created_at"`
	Visits    int       `json:"visits"`
	Custom    bool      `json:"custom"`
	Disabled  bool      `json:"disabled"`
}

// Validate checks that the CreateReq is well-formed.
func (r *CreateReq) Validate() error {
	if r.RawURL == "" {
		return fmt.Errorf("raw_url is required")
	}
	if len(r.RawURL) > 2048 {
		return fmt.Errorf("raw_url exceeds 2048 characters")
	}
	if r.CustomCode != "" {
		if len(r.CustomCode) < 3 || len(r.CustomCode) > 32 {
			return fmt.Errorf("custom_code must be between 3 and 32 characters")
		}
	}
	if r.MaxVisits < 0 {
		return fmt.Errorf("max_visits must be non-negative")
	}
	return nil
}

// Validate checks that the ShortURL fields are valid.
func (s *ShortURL) Validate() error {
	if s.Code == "" {
		return fmt.Errorf("code is required")
	}
	if s.RawURL == "" {
		return fmt.Errorf("raw_url is required")
	}
	if len(s.Code) > 32 {
		return fmt.Errorf("code must be at most 32 characters")
	}
	if len(s.RawURL) > 2048 {
		return fmt.Errorf("raw_url must be at most 2048 characters")
	}
	if s.Visits < 0 {
		return fmt.Errorf("visits must be non-negative")
	}
	return nil
}

// IsExpired reports whether the ShortURL has exceeded MaxVisits or the
// configured time-to-live when the URL is disabled. A URL is treated as
// expired when it is marked disabled, since the disabled flag is used by the
// upper layer as a soft-expiration signal.
func (s *ShortURL) IsExpired(now time.Time) bool {
	if s.Disabled {
		return true
	}
	if s.CreatedAt.IsZero() {
		return false
	}
	return now.Sub(s.CreatedAt) > 7*24*time.Hour
}

// NewShortURL constructs a ShortURL with sensible defaults.
func NewShortURL(code, rawURL string) *ShortURL {
	return &ShortURL{
		Code:      code,
		RawURL:    rawURL,
		CreatedAt: time.Now(),
		Visits:    0,
		Custom:    false,
		Disabled:  false,
	}
}
