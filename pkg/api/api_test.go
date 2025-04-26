package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthCheckHandler(t *testing.T) {
	// Create an instance of the API
	api := New(":8082")

	// Create a test HTTP request
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	// Create a ResponseRecorder to capture the response
	rr := httptest.NewRecorder()

	// Call the health check handler
	handler := http.HandlerFunc(api.healthCheckHandler)
	handler.ServeHTTP(rr, req)

	// Assert the status code
	assert.Equal(t, http.StatusOK, rr.Code, "Expected status code 200")

	// Assert the response body
	expectedResponse := map[string]string{"status": "healthy"}
	var actualResponse map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &actualResponse)
	assert.NoError(t, err, "Response body should be valid JSON")
	assert.Equal(t, expectedResponse, actualResponse, "Response body does not match expected")
}
