package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockResponseWriter struct {
	http.ResponseWriter
}

func (m *mockResponseWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("mock encoding error")
}

func TestHealthCheckHandler(t *testing.T) {
	api := New(Config{Listen: ":8082"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
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
	api := New(Config{Listen: ":8082"}, nil)
	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	mockWriter := &mockResponseWriter{ResponseWriter: httptest.NewRecorder()}
	handler := http.HandlerFunc(api.healthCheckHandler)

	handler.ServeHTTP(mockWriter, req)
	t.Logf("The test did not panic")
}

func TestGenerateTokenHandler(t *testing.T) {
	api := New(Config{
		DefaultTokenExpiry: 3600, // 1 hour
	}, nil)

	t.Run("Invalid Request Payload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/generateToken", bytes.NewBuffer([]byte("invalid json")))
		rec := httptest.NewRecorder()

		api.generateTokenHandler(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "Bad Request")
	})

	t.Run("Missing KeyID", func(t *testing.T) {
		requestBody := GenerateTokenRequest{
			TTL: 3600,
		}
		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest(http.MethodPost, "/generateToken", bytes.NewBuffer(body))
		rec := httptest.NewRecorder()

		api.generateTokenHandler(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response GenerateTokenResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response.Success)
		assert.NotEmpty(t, response.KeyID)
		assert.NotEmpty(t, response.Token)
		assert.Equal(t, uint(3600), response.TTL)
	})

	t.Run("Valid Request with KeyID", func(t *testing.T) {
		requestBody := GenerateTokenRequest{
			KeyID: "test-key-id",
			TTL:   3601,
		}
		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest(http.MethodPost, "/generateToken", bytes.NewBuffer(body))
		rec := httptest.NewRecorder()

		api.generateTokenHandler(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response GenerateTokenResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response.Success)
		assert.Equal(t, "test-key-id", response.KeyID)
		assert.NotEmpty(t, response.Token)
		assert.Equal(t, uint(3601), response.TTL)
	})

	t.Run("Invalid TTL", func(t *testing.T) {
		requestBody := GenerateTokenRequest{
			KeyID: "test-key-id",
			TTL:   0,
		}
		_api := New(Config{
			DefaultTokenExpiry: 0,
		}, nil)
		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest(http.MethodPost, "/generateToken", bytes.NewBuffer(body))
		rec := httptest.NewRecorder()

		_api.generateTokenHandler(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var response GenerateTokenResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.False(t, response.Success)
		assert.Contains(t, response.Message, "TTL must be greater than 0")
	})
}

func TestAPIRun(t *testing.T) {
	api := New(Config{Listen: ":8083"}, nil)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		err := api.Run(ctx)
		assert.NoError(t, err, "API server should shut down gracefully")
	}()

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://localhost:8083/health")

	assert.NoError(t, err, "Health check request should not return an error")
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Health check should return status 200")
	assert.Equal(t, resp.Header.Get("Content-Type"), "application/json", "Content-Type should be application/json")

	var response map[string]string
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err, "Response body should be valid JSON")
	assert.Equal(t, response["status"], "healthy", "Response body should contain status 'healthy'")

	defer resp.Body.Close()
	cancel()
}
