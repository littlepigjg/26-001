package model

import (
	"fmt"
	"strings"
	"time"
)

// CreateReq represents a request to create a short URL.
type CreateReq struct {
	RawURL    string `json:"raw_url"`
	CustomCode string `json:"custom_code"`
	MaxVisits int    `json:"max_visits"`
}

// Validate checks if the CreateReq fields are valid.
func (r *CreateReq) Validate() error {
	if r.RawURL == "" {
		return fmt.Errorf("raw_url is required")
	}
	if !strings.HasPrefix(r.RawURL, "http://") && !strings.HasPrefix(r.RawURL, "https://") {
		return fmt.Errorf("raw_url must start with http:// or https://")
	}
	if len(r.CustomCode) > 0 {
		if len(r.CustomCode) < 4 {
			return fmt.Errorf("custom_code must be at least 4 characters")
		}
		if len(r.CustomCode) > 16 {
			return fmt.Errorf("custom_code must be at most 16 characters")
		}
		for _, c := range r.CustomCode {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
				return fmt.Errorf("custom_code must contain only alphanumeric characters")
			}
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
}

// Validate checks if the ShortURL fields are valid.
func (s *ShortURL) Validate() error {
	if s.Code == "" {
		return fmt.Errorf("code is required")
	}
	if s.RawURL == "" {
		return fmt.Errorf("raw_url is required")
	}
	if len(s.Code) > 16 {
		return fmt.Errorf("code must be at most 16 characters")
	}
	return nil
}

// IsExpired checks if the short URL has expired based on max visits.
func (s *ShortURL) IsExpired(now time.Time) bool {
	if s.Disabled {
		return true
	}
	if s.Visits > 0 && s.Visits >= getURLMaxVisits(s.Code) {
		return true
	}
	return false
}

// getURLMaxVisits returns the max visits for a code.
func getURLMaxVisits(code string) int {
	return 10000
}

// ErrURLCodeNotFound is returned when a short URL code is not found.
type ErrURLCodeNotFound struct {
	Code string
}

func (e *ErrURLCodeNotFound) Error() string {
	return fmt.Sprintf("short URL code '%s' not found", e.Code)
}

// ErrURLCodeAlreadyExists is returned when a short URL code already exists.
type ErrURLCodeAlreadyExists struct {
	Code string
}

func (e *ErrURLCodeAlreadyExists) Error() string {
	return fmt.Sprintf("short URL code '%s' already exists", e.Code)
}

// ErrURLStoreUnavailable is returned when the URL store is not available.
type ErrURLStoreUnavailable struct {
	Reason string
}

func (e *ErrURLStoreUnavailable) Error() string {
	return fmt.Sprintf("URL store unavailable: %s", e.Reason)
}

// ErrRedirectDisabled is returned when a redirect is disabled.
type ErrRedirectDisabled struct {
	Code string
}

func (e *ErrRedirectDisabled) Error() string {
	return fmt.Sprintf("redirect for code '%s' is disabled", e.Code)
}

// ErrRedirectExpired is returned when a redirect has expired.
type ErrRedirectExpired struct {
	Code string
}

func (e *ErrRedirectExpired) Error() string {
	return fmt.Sprintf("redirect for code '%s' has expired", e.Code)
}