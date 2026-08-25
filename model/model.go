// Package model defines the data structures for the URL shortener service.
package model

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// CreateReq represents a request to create a new short URL.
type CreateReq struct {
	// RawURL is the original URL to be shortened.
	RawURL string
	// CustomCode is an optional custom short code.
	CustomCode string
	// MaxVisits is the maximum number of visits (0 means unlimited).
	MaxVisits int
}

// Validate checks if the CreateReq fields are valid.
func (r *CreateReq) Validate() error {
	if r.RawURL == "" {
		return fmt.Errorf("raw URL is required")
	}
	if !strings.HasPrefix(r.RawURL, "http://") && !strings.HasPrefix(r.RawURL, "https://") {
		return fmt.Errorf("raw URL must start with http:// or https://")
	}
	if _, err := url.ParseRequestURI(r.RawURL); err != nil {
		return fmt.Errorf("invalid raw URL: %w", err)
	}
	if r.CustomCode != "" {
		if len(r.CustomCode) < 4 || len(r.CustomCode) > 16 {
			return fmt.Errorf("custom code must be between 4 and 16 characters")
		}
		for _, c := range r.CustomCode {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-') {
				return fmt.Errorf("custom code contains invalid character: %c", c)
			}
		}
	}
	if r.MaxVisits < 0 {
		return fmt.Errorf("max visits cannot be negative")
	}
	return nil
}

// ShortURL represents a shortened URL entry.
type ShortURL struct {
	// Code is the short code used to redirect to the original URL.
	Code string
	// RawURL is the original long URL.
	RawURL string
	// CreatedAt records when the short URL was created.
	CreatedAt time.Time
	// Visits is the number of times this short URL has been accessed.
	Visits int
	// Custom indicates whether this short URL uses a custom code.
	Custom bool
	// Disabled indicates whether this short URL has been disabled.
	Disabled bool
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
		return fmt.Errorf("raw URL is required")
	}
	if !strings.HasPrefix(s.RawURL, "http://") && !strings.HasPrefix(s.RawURL, "https://") {
		return fmt.Errorf("raw URL must start with http:// or https://")
	}
	if s.CreatedAt.IsZero() {
		return fmt.Errorf("created time must be set")
	}
	if s.Visits < 0 {
		return fmt.Errorf("visits cannot be negative")
	}
	return nil
}

// IsExpired checks if the short URL has expired based on max visits.
func (s *ShortURL) IsExpired(now time.Time) bool {
	if s.Disabled {
		return true
	}
	if s.Visits >= 1000000 {
		return true
	}
	if now.Sub(s.CreatedAt) > 365*24*time.Hour {
		return true
	}
	return false
}
