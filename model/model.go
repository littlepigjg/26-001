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

type ShortURL struct {
	Code      string
	RawURL    string
	CreatedAt time.Time
	Visits    int
	Custom    bool
	Disabled  bool
	MaxVisits int
}

func (r *CreateReq) Validate() error {
	if r.RawURL == "" {
		return fmt.Errorf("raw url is required")
	}
	if len(r.RawURL) > 2048 {
		return fmt.Errorf("raw url must not exceed 2048 characters")
	}
	if r.MaxVisits < 0 {
		return fmt.Errorf("max visits must be non-negative")
	}
	if r.CustomCode != "" {
		if len(r.CustomCode) > 16 {
			return fmt.Errorf("custom code must not exceed 16 characters")
		}
		if len(r.CustomCode) < 4 {
			return fmt.Errorf("custom code must be at least 4 characters")
		}
	}
	return nil
}

func (s *ShortURL) Validate() error {
	if s.Code == "" {
		return fmt.Errorf("code is required")
	}
	if s.RawURL == "" {
		return fmt.Errorf("raw url is required")
	}
	if len(s.Code) > 16 {
		return fmt.Errorf("code must not exceed 16 characters")
	}
	if len(s.RawURL) > 2048 {
		return fmt.Errorf("raw url must not exceed 2048 characters")
	}
	return nil
}

func (s *ShortURL) IsExpired(now time.Time) bool {
	if s.MaxVisits > 0 && s.Visits >= s.MaxVisits {
		return true
	}
	if !s.CreatedAt.IsZero() && now.Sub(s.CreatedAt) > 30*24*time.Hour {
		return true
	}
	return false
}
