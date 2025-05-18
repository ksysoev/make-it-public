package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

type respWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader sets the HTTP status code for the response and writes it to the underlying ResponseWriter.
func (rw *respWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

// Metrics wraps an HTTP handler to log request details such as duration, status code, and path.
// It returns a middleware handler function that records metrics for incoming requests.
// Returns an HTTP handler middleware for logging request metrics.
func Metrics() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			now := time.Now()

			rw := &respWriter{
				ResponseWriter: w,
				status:         http.StatusOK, // Initialize with a default status.
			}

			next.ServeHTTP(rw, r)
			slog.InfoContext(r.Context(), "mng api request", slog.Duration("duration", time.Since(now)), slog.Int("status", rw.status), slog.String("path", r.URL.Path))
		})
	}
}
