package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
)

// clientIPKeyType is a custom type used as a key for storing client IP in the request context
type clientIPKeyType struct{}

// ClientIP is a middleware that identifies the client IP address from an HTTP request
// and stores it in the request context. It checks various headers including forwarded headers,
// Cloudflare, and CloudFront headers. If no headers are present, it falls back to the remote IP.
// Returns a middleware function that adds the client IP to the request context.
func ClientIP() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := extractClientIP(r)

			ctx := r.Context()
			ctx = context.WithValue(ctx, clientIPKeyType{}, clientIP)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

// GetClientIP retrieves the client IP address from the request context.
// Returns the client IP as a string, or an empty string if not found.
func GetClientIP(r *http.Request) string {
	if clientIP, ok := r.Context().Value(clientIPKeyType{}).(string); ok {
		return clientIP
	}

	return ""
}

// extractClientIP extracts the client IP address from various headers in the request.
// It checks headers in the following order:
// 1. CF-Connecting-IP (Cloudflare)
// 2. X-Forwarded-For
// 3. X-Real-IP
// 4. X-Forwarded
// 5. X-Cluster-Client-IP
// 6. True-Client-IP
// 7. X-CloudFront-Forwarded-For (AWS CloudFront)
// If no headers are present, it falls back to the remote IP from the request.
func extractClientIP(r *http.Request) string {
	// Check Cloudflare header
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}

	// Check X-Forwarded-For header
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := strings.Split(forwardedFor, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check other common headers
	headers := []string{
		"X-Real-IP",
		"X-Forwarded",
		"X-Cluster-Client-IP",
		"True-Client-IP",
	}

	for _, header := range headers {
		if ip := r.Header.Get(header); ip != "" {
			return ip
		}
	}

	// Check CloudFront header
	if cloudFrontIP := r.Header.Get("X-CloudFront-Forwarded-For"); cloudFrontIP != "" {
		ips := strings.Split(cloudFrontIP, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Fall back to remote address
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// If there's an error splitting the address, just return the RemoteAddr as is
		return r.RemoteAddr
	}

	return ip
}
