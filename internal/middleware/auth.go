package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"config-center/pkg/logger"
)

// RateLimiter implements a simple token bucket rate limiter.
// It limits the number of requests per client IP within a time window.
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitorInfo
	rate     int           // max requests per window
	window   time.Duration
	logger   *logger.Logger
}

// visitorInfo tracks request counts per client.
type visitorInfo struct {
	count    int
	lastSeen time.Time
}

// NewRateLimiter creates a new RateLimiter.
func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitorInfo),
		rate:     rate,
		window:   window,
		logger:   logger.WithField("middleware", "ratelimit"),
	}
	// Start cleanup goroutine
	go rl.cleanupLoop()
	return rl
}

// Allow checks if a request from the given IP is allowed.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	v, exists := rl.visitors[ip]
	if !exists || now.Sub(v.lastSeen) > rl.window {
		rl.visitors[ip] = &visitorInfo{count: 1, lastSeen: now}
		return true
	}
	v.count++
	v.lastSeen = now
	if v.count > rl.rate {
		return false
	}
	return true
}

// cleanupLoop periodically removes expired visitor entries.
func (rl *RateLimiter) cleanupLoop() {
	for {
		time.Sleep(rl.window)
		rl.mu.Lock()
		now := time.Now()
		for ip, v := range rl.visitors {
			if now.Sub(v.lastSeen) > rl.window*2 {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimitMiddleware returns a middleware that applies rate limiting.
func RateLimitMiddleware(rate int, window time.Duration) func(http.Handler) http.Handler {
	limiter := NewRateLimiter(rate, window)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getIP(r)
			if !limiter.Allow(ip) {
				logger.Warnf("rate limit exceeded for IP: %s", ip)
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// getIP extracts the client IP from the request.
func getIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		return ip
	}
	return r.RemoteAddr
}

// AuthMiddleware provides a simple authentication mechanism.
// It checks for a bearer token in the Authorization header.
type AuthMiddleware struct {
	mu     sync.RWMutex
	tokens map[string]string // token -> username
	logger *logger.Logger
}

// NewAuthMiddleware creates a new AuthMiddleware.
func NewAuthMiddleware() *AuthMiddleware {
	return &AuthMiddleware{
		tokens: make(map[string]string),
		logger: logger.WithField("middleware", "auth"),
	}
}

// AddToken registers an authentication token.
func (am *AuthMiddleware) AddToken(token, username string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.tokens[token] = username
}

// RemoveToken removes an authentication token.
func (am *AuthMiddleware) RemoveToken(token string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	delete(am.tokens, token)
}

// Validate checks if a token is valid and returns the associated username.
func (am *AuthMiddleware) Validate(token string) (string, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	username, ok := am.tokens[token]
	return username, ok
}

// Middleware returns a middleware that validates bearer tokens.
func (am *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			// Allow unauthenticated requests for health checks
			if r.URL.Path == "/health" || r.URL.Path == "/ready" {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "authorization required", http.StatusUnauthorized)
			return
		}

		// Extract bearer token
		token := ""
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token = authHeader[7:]
		} else {
			token = authHeader
		}

		username, valid := am.Validate(token)
		if !valid {
			http.Error(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Add user to context
		ctx := context.WithValue(r.Context(), ContextKeyUser, username)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Ensure sync import is used
