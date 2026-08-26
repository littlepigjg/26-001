package model

import "time"

// RedirectRequest is the input payload for a redirect lookup.
type RedirectRequest struct {
	Code      string
	Timestamp time.Time
}

// RedirectResult is the outcome of a redirect lookup.
type RedirectResult struct {
	RawURL string
	Status int
}
