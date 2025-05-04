package middleware

import (
	"context"
	"net/http"
	"strings"
)

type keyIDKeyType struct{}

// ParseKeyID checks if the request's host ends with the specified domain postfix and validates the subdomain.
// It rejects requests with unmatched postfix or missing subdomains by returning a 404 response.
// Accepts domainPostfix as a string specifying the desired domain suffix.
// Returns a middleware handler function that attaches the subdomain to the request context and processes the next handler.
func ParseKeyID(domainPostfix string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := strings.Split(r.Host, ":")[0]
			if !strings.HasSuffix(host, domainPostfix) {
				http.NotFound(w, r)
				return
			}

			keyID := getUserIDFromRequest(r)
			if keyID == "" {
				http.NotFound(w, r)
				return
			}

			ctx := r.Context()
			ctx = context.WithValue(ctx, keyIDKeyType{}, keyID)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

// GetKeyID retrieves the key ID from the request's context if available.
// It returns the key ID as a string, or an empty string if not found or the value is not a string.
func GetKeyID(r *http.Request) string {
	if keyID, ok := r.Context().Value(keyIDKeyType{}).(string); ok {
		return keyID
	}

	return ""
}

// getUserIDFromRequest extracts the subdomain from the host in the HTTP request.
// It assumes the host follows the subdomain.domain.tld format.
// Returns the subdomain as a string or an empty string if no subdomain exists.
func getUserIDFromRequest(r *http.Request) string {
	host := r.Host

	if host != "" {
		parts := strings.Split(host, ".")
		if len(parts) > 2 {
			// Extract subdomain (assuming subdomain.domain.tld format)
			return parts[0]
		}
	}

	return ""
}
