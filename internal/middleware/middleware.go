// Package middleware provides HTTP middleware functions for the configuration center.
// It includes recovery, logging, CORS, and authentication middleware.
package middleware

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"config-center/pkg/logger"
	"config-center/pkg/response"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	// ContextKeyRequestID is the context key for the request ID.
	ContextKeyRequestID contextKey = "request_id"
	// ContextKeyUser is the context key for the authenticated user.
	ContextKeyUser contextKey = "user"
	// ContextKeyStartTime is the context key for request start time.
	ContextKeyStartTime contextKey = "start_time"
)

// Recovery is a middleware that recovers from panics and returns a 500 error.
// It logs the panic details including stack trace.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()
				logger.Errorf("panic recovered: %v\n%s", err, stack)
				response.InternalError(w, fmt.Sprintf("internal server error: %v", err))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Logging is a middleware that logs HTTP request details including method, path, status code, and duration.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response recorder to capture the status code
		rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}

		// Add start time to context
		ctx := context.WithValue(r.Context(), ContextKeyStartTime, start)
		r = r.WithContext(ctx)

		next.ServeHTTP(rec, r)

		duration := time.Since(start)
		logger.Infof("HTTP %s %s %d %v", r.Method, r.URL.Path, rec.statusCode, duration)
	})
}

// CORS is a middleware that adds Cross-Origin Resource Sharing headers.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, If-None-Match")
		w.Header().Set("Access-Control-Expose-Headers", "ETag, Cache-Control")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RequestContext stores request-scoped information.
type RequestContext struct {
	// RequestID is a unique identifier for this request.
	RequestID string
	// User is the authenticated user (may be empty).
	User string
	// IPAddress is the client IP address.
	IPAddress string
	// StartedAt is when the request began processing.
	StartedAt time.Time
}

// NewRequestContextFromContext extracts a RequestContext from a context.
func NewRequestContextFromContext(ctx context.Context) *RequestContext {
	rc := &RequestContext{}
	if id, ok := ctx.Value(ContextKeyRequestID).(string); ok {
		rc.RequestID = id
	}
	if user, ok := ctx.Value(ContextKeyUser).(string); ok {
		rc.User = user
	}
	if start, ok := ctx.Value(ContextKeyStartTime).(time.Time); ok {
		rc.StartedAt = start
	}
	return rc
}

// responseRecorder wraps http.ResponseWriter to capture the status code.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

// WriteHeader captures the status code and delegates to the wrapped writer.
func (rec *responseRecorder) WriteHeader(code int) {
	if !rec.written {
		rec.statusCode = code
		rec.written = true
	}
	rec.ResponseWriter.WriteHeader(code)
}

// Write captures the first write and marks as written.
func (rec *responseRecorder) Write(b []byte) (int, error) {
	if !rec.written {
		rec.written = true
	}
	return rec.ResponseWriter.Write(b)
}
