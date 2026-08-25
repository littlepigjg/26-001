package model

import "errors"

// ShortURL-specific sentinel errors for errors.Is / errors.As support.
// These are returned wrapped by lower layers; callers should use errors.Is
// to detect them in the returned error chain.
var (
	ErrShortURLNotFound  = errors.New("short_url not found")
	ErrShortURLExists    = errors.New("short_url code already exists")
	ErrInvalidCode       = errors.New("invalid short_code")
	ErrAccessLogNotFound = errors.New("access_log not found")
	ErrVersionNotFound   = errors.New("version not found")
)
