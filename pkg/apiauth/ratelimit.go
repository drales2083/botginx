package apiauth

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

// RateLimiter implements a sliding window rate limiter
type RateLimiter struct {
	mu      sync.RWMutex
	windows map[string]*slidingWindow
	cleanup *time.Ticker
}

type slidingWindow struct {
	requests []time.Time
	limit    int
}

// NewRateLimiter creates a new rate limiter with automatic cleanup
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		windows: make(map[string]*slidingWindow),
		cleanup: time.NewTicker(5 * time.Minute),
	}
	go rl.cleanupLoop()
	return rl
}

// cleanupLoop periodically removes stale entries
func (rl *RateLimiter) cleanupLoop() {
	for range rl.cleanup.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-time.Hour)
		for key, window := range rl.windows {
			// Remove entries with no recent requests
			if len(window.requests) == 0 {
				delete(rl.windows, key)
				continue
			}
			// Check if latest request is older than 1 hour
			if window.requests[len(window.requests)-1].Before(cutoff) {
				delete(rl.windows, key)
			}
		}
		rl.mu.Unlock()
	}
}

// Check returns remaining requests and whether the request is allowed
// keyID is the API key ID, limit is requests per hour
func (rl *RateLimiter) Check(keyID string, limit int) (remaining int, resetAt time.Time, allowed bool) {
	now := time.Now()
	windowStart := now.Add(-time.Hour)
	resetAt = now.Add(time.Hour)

	rl.mu.Lock()
	defer rl.mu.Unlock()

	window, exists := rl.windows[keyID]
	if !exists {
		window = &slidingWindow{
			requests: make([]time.Time, 0),
			limit:    limit,
		}
		rl.windows[keyID] = window
	}

	// Update limit if changed
	window.limit = limit

	// Filter out requests outside the sliding window
	validRequests := make([]time.Time, 0, len(window.requests))
	for _, t := range window.requests {
		if t.After(windowStart) {
			validRequests = append(validRequests, t)
		}
	}
	window.requests = validRequests

	// Calculate remaining
	used := len(window.requests)
	remaining = limit - used

	if remaining <= 0 {
		// Find when the oldest request expires
		if len(window.requests) > 0 {
			resetAt = window.requests[0].Add(time.Hour)
		}
		return 0, resetAt, false
	}

	// Record this request
	window.requests = append(window.requests, now)
	return remaining - 1, resetAt, true
}

// RateLimitMiddleware returns middleware that enforces rate limits
func (rl *RateLimiter) RateLimitMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := GetAPIKey(r)
			if key == nil {
				// No API key in context, skip rate limiting
				next.ServeHTTP(w, r)
				return
			}

			remaining, resetAt, allowed := rl.Check(key.KeyID, key.RateLimit)

			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(key.RateLimit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))

			if !allowed {
				ErrorResp(w, "rate_limit_exceeded", "Rate limit exceeded. Try again later.", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Stop cleans up the rate limiter
func (rl *RateLimiter) Stop() {
	rl.cleanup.Stop()
}
