package model

import (
	"fmt"
	"time"
)

// CreateReq represents a request to create a short URL.
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

// Validate checks if CreateReq fields are valid.
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
		for _, c := range r.CustomCode {
			if !isValidCodeChar(c) {
				return fmt.Errorf("custom_code contains invalid character: %c", c)
			}
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
	if len(s.Code) < 4 || len(s.Code) > 16 {
		return fmt.Errorf("code must be between 4 and 16 characters")
	}
	if s.RawURL == "" {
		return fmt.Errorf("raw_url is required")
	}
	if len(s.RawURL) > 2048 {
		return fmt.Errorf("raw_url must be at most 2048 characters")
	}
	return nil
}

// IsExpired checks if the short URL has expired.
func (s *ShortURL) IsExpired(now time.Time) bool {
	if s.Disabled {
		return true
	}
	if now.Sub(s.CreatedAt) > 365*24*time.Hour {
		return true
	}
	return false
}

// isValidCodeChar checks if a character is valid for URL codes.
func isValidCodeChar(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_'
}
