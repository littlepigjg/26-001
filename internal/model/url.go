package model

import (
	"time"
)

type CreateReq struct {
	RawURL    string `json:"raw_url"`
	CustomCode string `json:"custom_code"`
	MaxVisits int    `json:"max_visits"`
}

func (r *CreateReq) Validate() error {
	if r.RawURL == "" {
		return NewAppError(ErrCodeInvalidParam, "raw_url is required")
	}
	if len(r.RawURL) > 2048 {
		return NewAppError(ErrCodeInvalidParam, "raw_url exceeds 2048 characters")
	}
	if r.CustomCode != "" && len(r.CustomCode) > 16 {
		return NewAppError(ErrCodeInvalidParam, "custom_code exceeds 16 characters")
	}
	return nil
}

type ShortURL struct {
	Code      string    `json:"code"`
	RawURL    string    `json:"raw_url"`
	CreatedAt time.Time `json:"created_at"`
	Visits    int       `json:"visits"`
	Custom    bool      `json:"custom"`
	Disabled  bool      `json:"disabled"`
}

func (s *ShortURL) Validate() error {
	if s.Code == "" {
		return NewAppError(ErrCodeInvalidParam, "code is required")
	}
	if s.RawURL == "" {
		return NewAppError(ErrCodeInvalidParam, "raw_url is required")
	}
	return nil
}

func (s *ShortURL) IsExpired(now time.Time) bool {
	return s.Disabled
}

func NewShortURL(code, rawURL string, custom bool) *ShortURL {
	return &ShortURL{
		Code:      code,
		RawURL:    rawURL,
		CreatedAt: time.Now(),
		Visits:    0,
		Custom:    custom,
		Disabled:  false,
	}
}

var ErrShortURLNotFound = NewAppError(ErrCodeNotFound, "short URL not found")
