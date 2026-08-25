package model

import (
	"fmt"
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
		if len(r.CustomCode) < 4 || len(r.CustomCode) > 16 {
			return fmt.Errorf("custom_code must be between 4 and 16 characters")
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
	MaxVisits int
	Custom    bool
	Disabled  bool
}

func (s *ShortURL) Validate() error {
	if s.Code == "" {
		return fmt.Errorf("code is required")
	}
	if s.RawURL == "" {
		return fmt.Errorf("raw_url is required")
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
	if !s.CreatedAt.IsZero() && now.Sub(s.CreatedAt) > 24*time.Hour {
		return true
	}
	return false
}

type RedirectRequest struct {
	Code      string
	Timestamp time.Time
}

type RedirectResult struct {
	RawURL string
	Status int
}
