package middleware

import (
	"net/http"

	"github.com/mileusna/useragent"
)

const (
	consentCookieName = "consent"
)

func NewFishingProtection() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ua := useragent.Parse(r.UserAgent())

			if ua.IsUnknown() {
				next.ServeHTTP(w, r)

				return
			}

			http.Error(w, "Fishing protection triggered", http.StatusForbidden)
		})
	}
}
