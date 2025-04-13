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
			result := server.getUserIDFromRequest(req)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestServeHTTP(t *testing.T) {
	tests := []struct {
		mockSetup      func(*MockConnService)
		name           string
		host           string
		expectedBody   string
		expectedStatus int
	}{
		{
			name:           "invalid domain",
			host:           "invalid.com",
			mockSetup:      nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "request is not sent to the defined domain\n",
		},
		{
			name:           "missing subdomain",
			host:           "example.com",
			mockSetup:      nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "invalid or missing subdomain\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockConnService := NewMockConnService(t)
			if tt.mockSetup != nil {
				tt.mockSetup(mockConnService)
			}

			server := &HTTPServer{
				connService: mockConnService,
				config: Config{
					Domain: "example.com",
				},
			}

			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.Host = tt.host

			// Mock HTTP hijacker response writer
			rec := httptest.NewRecorder()

			// Act
			server.ServeHTTP(rec, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, rec.Code)
			assert.Equal(t, tt.expectedBody, rec.Body.String())

			if tt.mockSetup != nil {
				mockConnService.AssertExpectations(t)
			}
		})
	}
}
