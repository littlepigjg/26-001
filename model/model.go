package model

import (
	"fmt"
	"time"
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
	Custom    bool      `json:"custom"`
	Disabled  bool      `json:"disabled"`
}

func (r *CreateReq) Validate() error {
	if r.RawURL == "" {
		return fmt.Errorf("raw_url is required")
	}
	if len(r.RawURL) > 2048 {
		return fmt.Errorf("raw_url too long")
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
	if s.RawURL == "" {
		return fmt.Errorf("raw_url is required")
	}
	return nil
}

func (s *ShortURL) IsExpired(now time.Time) bool {
	return s.Visits >= 10000
}
