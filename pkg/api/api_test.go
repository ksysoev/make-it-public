package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockResponseWriter struct {
	http.ResponseWriter
}

func (m *mockResponseWriter) Write(b []byte) (int, error) {
	return 0, errors.New("mock encoding error")
}

func TestHealthCheckHandler(t *testing.T) {
	api := New(":8082")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(api.healthCheckHandler)

	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code, "Expected status code 200")

	expectedResponse := map[string]string{"status": "healthy"}

	var actualResponse map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &actualResponse)

	assert.NoError(t, err, "Response body should be valid JSON")
	assert.Equal(t, expectedResponse, actualResponse, "Response body does not match expected")
}

func TestHealthCheckHandler_JSONEncodeError(t *testing.T) {
	api := New(":8082")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	mockWriter := &mockResponseWriter{ResponseWriter: httptest.NewRecorder()}
	handler := http.HandlerFunc(api.healthCheckHandler)

	handler.ServeHTTP(mockWriter, req)
}
