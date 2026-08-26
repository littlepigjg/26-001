package model

import (
	"fmt"
	"regexp"
	"time"
)

var (
	codePattern   = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	urlPattern    = regexp.MustCompile(`^https?://.+`)
)

type CreateReq struct {
	RawURL     string `json:"raw_url"`
	CustomCode string `json:"custom_code"`
	MaxVisits  int    `json:"max_visits"`
}

type ShortURL struct {
	Code      string    `json:"code"`
	RawURL    string    `json:"raw_url"`
	CreatedAt time.Time `json:"created_at"`
	Visits    int       `json:"visits"`
	MaxVisits int       `json:"max_visits"`
	Custom    bool      `json:"custom"`
	Disabled  bool      `json:"disabled"`
}

type RedirectRequest struct {
	Code      string    `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}

type RedirectResult struct {
	RawURL string `json:"raw_url"`
	Status int    `json:"status"`
}

func (r *CreateReq) Validate() error {
	if r.RawURL == "" {
		return fmt.Errorf("raw_url is required")
	}
	if !urlPattern.MatchString(r.RawURL) {
		return fmt.Errorf("raw_url must start with http:// or https://")
	}
	if len(r.RawURL) > 2048 {
		return fmt.Errorf("raw_url must be at most 2048 characters")
	}
	if r.CustomCode != "" {
		if len(r.CustomCode) < 3 || len(r.CustomCode) > 32 {
			return fmt.Errorf("custom_code must be between 3 and 32 characters")
		}
		if !codePattern.MatchString(r.CustomCode) {
			return fmt.Errorf("custom_code must contain only letters, numbers, hyphens, and underscores")
		}
	}
	if r.MaxVisits < 0 {
		return fmt.Errorf("max_visits must be non-negative")
	}
	return nil
}

func (s *ShortURL) Validate() error {
	if s.Code == "" {
		return fmt.Errorf("code is required")
	}
	if !codePattern.MatchString(s.Code) {
		return fmt.Errorf("code must contain only letters, numbers, hyphens, and underscores")
	}
	if s.RawURL == "" {
		return fmt.Errorf("raw_url is required")
	}
	if !urlPattern.MatchString(s.RawURL) {
		return fmt.Errorf("raw_url must start with http:// or https://")
	}
	if len(s.RawURL) > 2048 {
		return fmt.Errorf("raw_url must be at most 2048 characters")
	}
	return nil
}

func (s *ShortURL) IsExpired(now time.Time) bool {
	if s.MaxVisits > 0 && s.Visits >= s.MaxVisits {
		return true
	}
	return false
}

func (s *ShortURL) IsDisabled() bool {
	return s.Disabled
}

func (s *ShortURL) IncrementVisits() {
	s.Visits++
}
