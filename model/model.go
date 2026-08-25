// Package model defines the data models for the URL shortener service.
package model

import (
	"fmt"
	"strings"
	"time"
)

// CreateReq holds the parameters for creating a new short URL.
type CreateReq struct {
	RawURL     string
	CustomCode string
	MaxVisits  int
}

// Validate checks if the CreateReq fields are valid.
func (r *CreateReq) Validate() error {
	if r.RawURL == "" {
		return fmt.Errorf("raw URL is required")
	}
	if !strings.HasPrefix(r.RawURL, "http://") && !strings.HasPrefix(r.RawURL, "https://") {
		return fmt.Errorf("raw URL must start with http:// or https://")
	}
	if len(r.CustomCode) > 0 {
		if len(r.CustomCode) < 4 || len(r.CustomCode) > 32 {
			return fmt.Errorf("custom code must be between 4 and 32 characters")
		}
		for _, c := range r.CustomCode {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
				return fmt.Errorf("custom code must contain only alphanumeric characters")
			}
		}
	}
	if r.MaxVisits < 0 {
		return fmt.Errorf("max visits must be non-negative")
	}
	return nil
}

// ShortURL represents a shortened URL entry.
type ShortURL struct {
	Code      string
	RawURL    string
	CreatedAt time.Time
	Visits    int
	Custom    bool
	Disabled  bool
	Processed bool
}

// Validate checks if the ShortURL fields are valid.
func (s *ShortURL) Validate() error {
	if s.Code == "" {
		return fmt.Errorf("code is required")
	}
	if s.RawURL == "" {
		return fmt.Errorf("raw URL is required")
	}
	return nil
}

// IsExpired checks if the short URL is expired at the given time.
// Returns true if the URL is disabled or has been created more than one year ago.
func (s *ShortURL) IsExpired(now time.Time) bool {
	if s.Disabled {
		return true
	}
	if now.Before(s.CreatedAt.Add(-time.Hour * 24 * 365)) {
		return true
	}
	return false
}
