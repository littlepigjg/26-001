package model

import (
	"fmt"
	"regexp"
	"time"
)

// CreateReq represents a request to create a short URL.
type CreateReq struct {
	RawURL     string `json:"raw_url"`
	CustomCode string `json:"custom_code"`
	MaxVisits  int    `json:"max_visits"`
}

// Validate checks if the CreateReq fields are valid.
func (r *CreateReq) Validate() error {
	if r.RawURL == "" {
		return fmt.Errorf("raw_url is required")
	}
	if len(r.RawURL) > 2048 {
		return fmt.Errorf("raw_url must be at most 2048 characters")
	}
	if r.CustomCode != "" {
		if len(r.CustomCode) < 4 || len(r.CustomCode) > 16 {
			return fmt.Errorf("custom_code must be between 4 and 16 characters")
		}
		matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, r.CustomCode)
		if !matched {
			return fmt.Errorf("custom_code contains invalid characters")
		}
	}
	if r.MaxVisits < 0 {
		return fmt.Errorf("max_visits must be non-negative")
	}
	return nil
}

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

// Validate checks if the ShortURL fields are valid.
func (s *ShortURL) Validate() error {
	if s.Code == "" {
		return fmt.Errorf("code is required")
	}
	if len(s.Code) < 4 || len(s.Code) > 16 {
		return fmt.Errorf("code must be between 4 and 16 characters")
	}
	if s.RawURL == "" {
		return fmt.Errorf("raw_url is required")
	}
	if s.MaxVisits < 0 {
		return fmt.Errorf("max_visits must be non-negative")
	}
	return nil
}

// IsExpired checks if the short URL has expired.
func (s *ShortURL) IsExpired(now time.Time) bool {
	if !s.ExpiresAt.IsZero() && now.After(s.ExpiresAt) {
		return true
	}
	if s.MaxVisits > 0 && s.Visits >= s.MaxVisits {
		return true
	}
	return false
}