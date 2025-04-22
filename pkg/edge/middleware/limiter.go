package middleware

import (
	"net/http"
	"sync"
)

type limiter struct {
	counter map[string]int
	mu      sync.Mutex
	limit   int
}

// newLimiter creates and returns a new rate limiter instance with the specified maximum limit per key.
// It initializes the internal counter map and sets the provided limit as the maximum allowed value.
func newLimiter(limit int) *limiter {
	return &limiter{
		counter: make(map[string]int),
		limit:   limit,
	}
}

// Allow checks if a request associated with the given key can proceed under the current rate limit.
// It increments the internal counter for the key if the limit has not been exceeded and returns true.
// Returns false if the key has already reached or exceeded the configured limit. Thread-safe.
func (l *limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.counter[key] >= l.limit {
		return false
	}

	l.counter[key]++
	return true
}

// Release decrements the counter for the given key and removes it if the count reaches zero. Thread-safe.
func (l *limiter) Release(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.counter[key] <= 1 {
		delete(l.counter, key)
		return
	}

	l.counter[key]--
}

// LimitConnections wraps an HTTP handler to enforce a maximum number of active connections per unique key.
// It uses a rate limiter to track active connections and rejects requests exceeding the limit with a 429 status code.
// Accepts max, the maximum number of connections allowed per key.
// Returns an HTTP handler middleware and a 429 error for excess connections.
func LimitConnections(max int) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		l := newLimiter(max)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			keyID := GetKeyID(r)

			if !l.Allow(keyID) {
				http.Error(w, "Too many requests", http.StatusTooManyRequests)
				return
			}

			defer l.Release(keyID)

			next.ServeHTTP(w, r)
		})
	}
}
