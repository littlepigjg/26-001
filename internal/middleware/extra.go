// Package middleware provides additional HTTP middleware for the configuration center.
package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"config-center/pkg/logger"
)

// RequestIDMiddleware generates or forwards a request ID for each request.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)
		ctx := r.Context()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// generateRequestID creates a simple unique request ID.
func generateRequestID() string {
	return time.Now().Format("20060102150405.000000000")
}

// TimeoutMiddleware sets a timeout for each request.
func TimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequestSizeLimitMiddleware limits the maximum request body size.
func RequestSizeLimitMiddleware(maxSize int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxSize {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			if r.ContentLength == 0 && r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxSize)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ContentTypeMiddleware ensures that POST/PUT/PATCH requests have the correct Content-Type.
func ContentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
			contentType := r.Header.Get("Content-Type")
			if contentType != "" && !strings.HasPrefix(contentType, "application/json") {
				http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// MethodOverrideMiddleware allows overriding the HTTP method via X-HTTP-Method-Override header.
func MethodOverrideMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if override := r.Header.Get("X-HTTP-Method-Override"); override != "" {
			r.Method = override
		}
		next.ServeHTTP(w, r)
	})
}

// MaintenanceModeMiddleware returns a 503 response when the server is in maintenance mode.
func MaintenanceModeMiddleware(isMaintenance func() bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isMaintenance() {
				http.Error(w, "service is undergoing maintenance", http.StatusServiceUnavailable)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// LoggingEnhanced provides more detailed logging including request and response sizes.
func LoggingEnhanced(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rec := &loggingRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rec, r)

		duration := time.Since(start)
		logger.Infof("REQ %s %s %d %v %dB",
			r.Method, r.URL.Path, rec.statusCode, duration, rec.bytesWritten)
	})
}

// loggingRecorder records the response status and body size.
type loggingRecorder struct {
	http.ResponseWriter
	statusCode  int
	bytesWritten int
}

// WriteHeader captures the status code.
func (rr *loggingRecorder) WriteHeader(code int) {
	rr.statusCode = code
	rr.ResponseWriter.WriteHeader(code)
}

// Write captures the number of bytes written.
func (rr *loggingRecorder) Write(b []byte) (int, error) {
	n, err := rr.ResponseWriter.Write(b)
	rr.bytesWritten += n
	return n, err
}
