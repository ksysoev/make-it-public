package middleware

import (
	"log/slog"
	"net/http"
)

func Metrics() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			keyID := GetKeyID(r)
			slog.InfoContext(r.Context(), "connection to edge server", slog.String("key_id", keyID))

			next.ServeHTTP(w, r)
		})
	}
}
