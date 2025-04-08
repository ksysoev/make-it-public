package edge

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetUserIDFromHeader(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected string
	}{
		{"valid subdomain", "user123.example.com", "user123"},
		{"no subdomain", "example.com", ""},
		{"empty host", "", ""},
		{"multiple subdomains", "user.sub.example.com", "user"},
		{"invalid format with port", "example.com:8080", ""},
		{"valid with port", "user123.example.com:8080", "user123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.Host = tt.host
			server := &HTTPServer{}

			// Act
			result := server.getUserIDFromHeader(req)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}
