package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		expectedIP string
	}{
		{
			name:       "Remote IP only",
			remoteAddr: "192.168.1.1:1234",
			headers:    map[string]string{},
			expectedIP: "192.168.1.1",
		},
		{
			name:       "Cloudflare header",
			remoteAddr: "10.0.0.1:1234",
			headers: map[string]string{
				"CF-Connecting-IP": "203.0.113.1",
			},
			expectedIP: "203.0.113.1",
		},
		{
			name:       "X-Forwarded-For header",
			remoteAddr: "10.0.0.1:1234",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.2, 10.0.0.1",
			},
			expectedIP: "203.0.113.2",
		},
		{
			name:       "X-Real-IP header",
			remoteAddr: "10.0.0.1:1234",
			headers: map[string]string{
				"X-Real-IP": "203.0.113.3",
			},
			expectedIP: "203.0.113.3",
		},
		{
			name:       "CloudFront header",
			remoteAddr: "10.0.0.1:1234",
			headers: map[string]string{
				"X-CloudFront-Forwarded-For": "203.0.113.4, 10.0.0.1",
			},
			expectedIP: "203.0.113.4",
		},
		{
			name:       "Multiple headers - priority check",
			remoteAddr: "10.0.0.1:1234",
			headers: map[string]string{
				"CF-Connecting-IP":           "203.0.113.1",
				"X-Forwarded-For":            "203.0.113.2",
				"X-Real-IP":                  "203.0.113.3",
				"X-CloudFront-Forwarded-For": "203.0.113.4",
			},
			expectedIP: "203.0.113.1", // CF-Connecting-IP should have highest priority
		},
		{
			name:       "Invalid remote address",
			remoteAddr: "invalid-address",
			headers:    map[string]string{},
			expectedIP: "invalid-address", // Should return the original address if parsing fails
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test handler that will check the client IP
			var capturedIP string
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedIP = GetClientIP(r)
				w.WriteHeader(http.StatusOK)
			})

			// Apply the middleware
			middleware := ClientIP()
			handler := middleware(testHandler)

			// Create a test request
			req := httptest.NewRequest("GET", "http://example.com", nil)
			req.RemoteAddr = tt.remoteAddr

			// Add headers
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			// Create a response recorder
			rr := httptest.NewRecorder()

			// Serve the request
			handler.ServeHTTP(rr, req)

			// Check the status code
			if status := rr.Code; status != http.StatusOK {
				t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
			}

			// Check the client IP
			if capturedIP != tt.expectedIP {
				t.Errorf("GetClientIP() = %v, want %v", capturedIP, tt.expectedIP)
			}
		})
	}
}

func TestExtractClientIP(t *testing.T) {
	// Direct test of the extractClientIP function
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		expectedIP string
	}{
		{
			name:       "Remote IP only",
			remoteAddr: "192.168.1.1:1234",
			headers:    map[string]string{},
			expectedIP: "192.168.1.1",
		},
		{
			name:       "X-Forwarded-For with multiple IPs",
			remoteAddr: "10.0.0.1:1234",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.2, 10.0.0.1, 192.168.1.1",
			},
			expectedIP: "203.0.113.2",
		},
		{
			name:       "X-Forwarded header",
			remoteAddr: "10.0.0.1:1234",
			headers: map[string]string{
				"X-Forwarded": "203.0.113.5",
			},
			expectedIP: "203.0.113.5",
		},
		{
			name:       "X-Cluster-Client-IP header",
			remoteAddr: "10.0.0.1:1234",
			headers: map[string]string{
				"X-Cluster-Client-IP": "203.0.113.6",
			},
			expectedIP: "203.0.113.6",
		},
		{
			name:       "True-Client-IP header",
			remoteAddr: "10.0.0.1:1234",
			headers: map[string]string{
				"True-Client-IP": "203.0.113.7",
			},
			expectedIP: "203.0.113.7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test request
			req := httptest.NewRequest("GET", "http://example.com", nil)
			req.RemoteAddr = tt.remoteAddr

			// Add headers
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			// Call the function directly
			ip := extractClientIP(req)

			// Check the result
			if ip != tt.expectedIP {
				t.Errorf("extractClientIP() = %v, want %v", ip, tt.expectedIP)
			}
		})
	}
}
