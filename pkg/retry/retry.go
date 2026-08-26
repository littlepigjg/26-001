// Package retry provides utilities for retrying operations with exponential backoff.
package retry

import (
	"context"
	"fmt"
	"math"
	"time"
)

// RetryConfig holds configuration for retry operations.
type RetryConfig struct {
	// MaxAttempts is the maximum number of retry attempts.
	MaxAttempts int
	// InitialDelay is the delay before the first retry.
	InitialDelay time.Duration
	// MaxDelay is the maximum delay between retries.
	MaxDelay time.Duration
	// BackoffMultiplier is the multiplier for exponential backoff.
	BackoffMultiplier float64
	// RetryableError is a function that determines if an error is retryable.
	// If nil, all errors are retried.
	RetryableError func(error) bool
}

// DefaultConfig returns a default retry configuration.
func DefaultConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      100 * time.Millisecond,
		MaxDelay:          30 * time.Second,
		BackoffMultiplier: 2.0,
		RetryableError:    nil,
	}
}

// Do executes an operation with retry logic.
// It retries the operation according to the configuration, respecting context cancellation.
func Do(ctx context.Context, cfg RetryConfig, operation func(ctx context.Context) error) error {
	var lastErr error

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if attempt > 0 {
			// Calculate delay with exponential backoff
			delay := float64(cfg.InitialDelay) * math.Pow(cfg.BackoffMultiplier, float64(attempt-1))
			if delay > float64(cfg.MaxDelay) {
				delay = float64(cfg.MaxDelay)
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(delay)):
			}
		}

		lastErr = operation(ctx)
		if lastErr == nil {
			return nil
		}

		// Check if error is retryable
		if cfg.RetryableError != nil && !cfg.RetryableError(lastErr) {
			return lastErr
		}

		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", cfg.MaxAttempts, lastErr)
}

// DoWithResult executes an operation that returns a value, with retry logic.
func DoWithResult(ctx context.Context, cfg RetryConfig, operation func(ctx context.Context) (interface{}, error)) (interface{}, error) {
	var lastErr error
	var lastResult interface{}

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if attempt > 0 {
			delay := float64(cfg.InitialDelay) * math.Pow(cfg.BackoffMultiplier, float64(attempt-1))
			if delay > float64(cfg.MaxDelay) {
				delay = float64(cfg.MaxDelay)
			}

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(delay)):
			}
		}

		lastResult, lastErr = operation(ctx)
		if lastErr == nil {
			return lastResult, nil
		}

		if cfg.RetryableError != nil && !cfg.RetryableError(lastErr) {
			return nil, lastErr
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	return nil, fmt.Errorf("operation failed after %d attempts: %w", cfg.MaxAttempts, lastErr)
}

// IsRetryable determines if an error should be retried based on common patterns.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	// Network errors, context deadlines, and temporary errors are retryable
	errStr := err.Error()
	return containsAny(errStr,
		"connection refused",
		"connection reset",
		"i/o timeout",
		"context deadline exceeded",
		"http2: server sent GOAWAY",
		"EOF",
		"temporarily",
	)
}

// containsAny checks if s contains any of the substrings.
func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) && containsSubstr(s, sub) {
			return true
		}
	}
	return false
}

// containsSubstr checks if s contains substr.
func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
