package middleware

import (
	"log/slog"
	"net/http"
)

// Metrics is an HTTP middleware that logs the "key_id" field from the request context
// and passes the request to the next handler. It is intended for use in edge server
// applications to track connections and associated keys.
func Metrics() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			keyID := GetKeyID(r)
			slog.InfoContext(r.Context(), "connection to edge server", slog.String("key_id", keyID))

			next.ServeHTTP(w, r)
		})
	}
}
