package model

import (
	"fmt"
	"regexp"
	"time"
)

type CreateReq struct {
	RawURL     string
	CustomCode string
	MaxVisits  int
}

func (r *CreateReq) Validate() error {
	if r.RawURL == "" {
		return fmt.Errorf("raw_url is required")
	}
	if len(r.RawURL) > 2048 {
		return fmt.Errorf("raw_url must be at most 2048 characters")
	}
	if r.CustomCode != "" {
		matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]{4,16}$`, r.CustomCode)
		if !matched {
			return fmt.Errorf("custom_code must be 4-16 characters, alphanumeric, hyphen, or underscore")
		}
	}
	if r.MaxVisits < 0 {
		return fmt.Errorf("max_visits must be non-negative")
	}
	return nil
}

type ShortURL struct {
	Code      string
	RawURL    string
	CreatedAt time.Time
	Visits    int
	Custom    bool
	Disabled  bool
	MaxVisits int
	ExpiresAt time.Time
}

func (s *ShortURL) Validate() error {
	if s.Code == "" {
		return fmt.Errorf("code is required")
	}
	if s.RawURL == "" {
		return fmt.Errorf("raw_url is required")
	}
	if s.Disabled {
		return fmt.Errorf("short url is disabled")
	}
	return nil
}

func (s *ShortURL) IsExpired(now time.Time) bool {
	if s.MaxVisits > 0 && s.Visits >= s.MaxVisits {
		return true
	}
	if !s.ExpiresAt.IsZero() && now.After(s.ExpiresAt) {
		return true
	}
	if s.CreatedAt.IsZero() {
		return false
	}
	return now.Sub(s.CreatedAt) > 30*24*time.Hour
}

type RedirectRequest struct {
	Code      string
	Timestamp time.Time
}

type RedirectResult struct {
	RawURL string
	Status int
}
