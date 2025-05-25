package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseKeyID(t *testing.T) {
	tests := []struct {
		name          string
		domainPostfix string
		host          string
		expectedKeyID string
		wantContent   string
		wantStatus    int
	}{
		{
			name:          "valid host with keyID",
			domainPostfix: ".example.com",
			host:          "keyID.example.com",
			expectedKeyID: "keyID",
			wantStatus:    http.StatusOK,
			wantContent:   "",
		},
		{
			name:          "invalid host without domain postfix",
			domainPostfix: ".example.com",
			host:          "keyID.notexample.com",
			expectedKeyID: "",
			wantStatus:    http.StatusNotFound,
			wantContent:   htmlErrorTemplate404,
		},
		{
			name:          "valid host without keyID",
			domainPostfix: ".example.com",
			host:          "example.com",
			expectedKeyID: "",
			wantStatus:    http.StatusNotFound,
			wantContent:   htmlErrorTemplate404,
		},
		{
			name:          "empty host",
			domainPostfix: ".example.com",
			host:          "",
			expectedKeyID: "",
			wantStatus:    http.StatusNotFound,
			wantContent:   htmlErrorTemplate404,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := ParseKeyID(tt.domainPostfix)

			// Create a dummy handler to validate the middleware
			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				keyID := GetKeyID(r)
				assert.Equal(t, tt.expectedKeyID, keyID)
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.Host = tt.host
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Result().StatusCode)
			assert.Equal(t, tt.wantContent, rec.Body.String())
		})
	}
}

func TestGetKeyID(t *testing.T) {
	tests := []struct {
		name          string
		contextValues map[interface{}]interface{}
		expectedKeyID string
	}{
		{
			name:          "keyID exists in context",
			contextValues: map[interface{}]interface{}{keyIDKeyType{}: "mockKeyID"},
			expectedKeyID: "mockKeyID",
		},
		{
			name:          "keyID missing in context",
			contextValues: map[interface{}]interface{}{},
			expectedKeyID: "",
		},
		{
			name:          "context value is not a string",
			contextValues: map[interface{}]interface{}{keyIDKeyType{}: 1234},
			expectedKeyID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			for key, value := range tt.contextValues {
				ctx = context.WithValue(ctx, key, value)
			}

			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody).WithContext(ctx)
			actualKeyID := GetKeyID(req)

			assert.Equal(t, tt.expectedKeyID, actualKeyID)
		})
	}
}

func TestGetUserIDFromRequest(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected string
	}{
		{
			name:     "valid host with subdomain",
			host:     "keyID.example.com",
			expected: "keyID",
		},
		{
			name:     "host without subdomain",
			host:     "example.com",
			expected: "",
		},
		{
			name:     "empty host",
			host:     "",
			expected: "",
		},
		{
			name:     "host with multiple subdomains",
			host:     "keyID.sub.example.com",
			expected: "keyID",
		},
		{
			name:     "invalid host with multiple dots but no subdomain",
			host:     ".example.com",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.Host = tt.host

			actual := getUserIDFromRequest(req)

			assert.Equal(t, tt.expected, actual)
		})
	}
}
