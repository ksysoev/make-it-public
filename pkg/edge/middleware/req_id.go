package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// ReqID adds a unique request ID to the context of each incoming HTTP request for tracking and correlation.
// It generates a new UUID for every request and stores it in the context under the key "req_id".
// Returns a middleware function wrapping an http.Handler and an error if middleware logic fails.
func ReqID() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), "req_id", uuid.New().String())
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}
