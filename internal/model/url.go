package model

import (
	"fmt"
	"regexp"
	"time"
)

// MaxCodeLength is the maximum length of a short code.
const MaxCodeLength = 16

// MinCodeLength is the minimum length of a short code.
const MinCodeLength = 4

// ShortURLExpiryDuration is the default expiry time for short URLs.
const ShortURLExpiryDuration = 24 * time.Hour

// urlRegexp validates a raw URL string.
var urlRegexp = regexp.MustCompile(`^https?://.+`)

// ShortURL represents a shortened URL entry.
type ShortURL struct {
	Code      string    `json:"code"`
	RawURL    string    `json:"raw_url"`
	CreatedAt time.Time `json:"created_at"`
	Visits    int       `json:"visits"`
	Custom    bool      `json:"custom"`
	Disabled  bool      `json:"disabled"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	MaxVisits int       `json:"max_visits,omitempty"`
}

// CreateReq is the request to create a short URL.
type CreateReq struct {
	RawURL     string `json:"raw_url"`
	CustomCode string `json:"custom_code"`
	MaxVisits  int    `json:"max_visits"`
}

// Validate checks if CreateReq fields are valid.
func (r *CreateReq) Validate() error {
	if r.RawURL == "" {
		return fmt.Errorf("raw_url is required")
	}
	if !urlRegexp.MatchString(r.RawURL) {
		return fmt.Errorf("raw_url must start with http:// or https://")
	}
	if len(r.RawURL) > 2048 {
		return fmt.Errorf("raw_url must be at most 2048 characters")
	}
	if r.CustomCode != "" {
		if len(r.CustomCode) < MinCodeLength || len(r.CustomCode) > MaxCodeLength {
			return fmt.Errorf("custom_code must be between %d and %d characters", MinCodeLength, MaxCodeLength)
		}
	}
	if r.MaxVisits < 0 {
		return fmt.Errorf("max_visits must be non-negative")
	}
	return nil
}

// Validate checks if ShortURL fields are valid.
func (s *ShortURL) Validate() error {
	if s.Code == "" {
		return fmt.Errorf("code is required")
	}
	if len(s.Code) < MinCodeLength || len(s.Code) > MaxCodeLength {
		return fmt.Errorf("code must be between %d and %d characters", MinCodeLength, MaxCodeLength)
	}
	if s.RawURL == "" {
		return fmt.Errorf("raw_url is required")
	}
	if !urlRegexp.MatchString(s.RawURL) {
		return fmt.Errorf("raw_url must start with http:// or https://")
	}
	return nil
}

// IsExpired checks if the short URL has expired at the given time.
func (s *ShortURL) IsExpired(now time.Time) bool {
	if s.ExpiresAt.IsZero() {
		return false
	}
	return now.After(s.ExpiresAt)
}

// IsExhausted checks if the max visits limit has been reached.
func (s *ShortURL) IsExhausted() bool {
	return s.MaxVisits > 0 && s.Visits >= s.MaxVisits
}

// ToMap converts the ShortURL to a map representation.
func (s *ShortURL) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"code":       s.Code,
		"raw_url":    s.RawURL,
		"created_at": s.CreatedAt,
		"visits":     s.Visits,
		"custom":     s.Custom,
		"disabled":   s.Disabled,
		"expires_at": s.ExpiresAt,
	}
}

// NewShortURL creates a new ShortURL with sensible defaults.
func NewShortURL(code, rawURL string, custom bool) *ShortURL {
	now := time.Now()
	return &ShortURL{
		Code:      code,
		RawURL:    rawURL,
		CreatedAt: now,
		Visits:    0,
		Custom:    custom,
		Disabled:  false,
		ExpiresAt: now.Add(ShortURLExpiryDuration),
		MaxVisits: 0,
	}
}
