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

	"github.com/ksysoev/make-it-public/pkg/core"
	"github.com/ksysoev/make-it-public/pkg/core/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockResponseWriter struct {
	http.ResponseWriter
}

func (m *mockResponseWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("mock encoding error")
}

func TestHealthCheckHandler(t *testing.T) {
	svc := NewMockService(t)
	svc.EXPECT().CheckHealth(mock.Anything).Return(nil).Once()

	api := New(Config{Listen: ":8082"}, svc)
	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(api.healthCheckHandler)

	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code, "Expected status code 200")
	assert.Equal(t, rr.Body.String(), "healthy", "Response body does not match expected")
}

func TestHealthCheckHandler_Error(t *testing.T) {
	svc := NewMockService(t)
	svc.EXPECT().CheckHealth(mock.Anything).Return(assert.AnError).Once()

	api := New(Config{Listen: ":0"}, svc)
	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(api.healthCheckHandler)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code, "Expected status code 500")
	assert.Equal(t, rr.Body.String(), "Internal Server Error\n", "Response body does not match expected")
}

func TestGenerateTokenHandler(t *testing.T) {
	auth := NewMockService(t)
	api := New(Config{}, auth)

	t.Run("Invalid Request Payload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/token", bytes.NewBuffer([]byte("invalid json")))
		rec := httptest.NewRecorder()

		api.generateTokenHandler(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "Bad Request")
	})

	t.Run("Success token generation", func(t *testing.T) {
		auth.EXPECT().GenerateToken(mock.Anything, mock.Anything, 3600).Return(&token.Token{
			ID:     "random-key-id",
			Secret: "test-token",
			TTL:    time.Hour,
		}, nil).Once()

		requestBody := GenerateTokenRequest{
			TTL: 3600,
		}
		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest(http.MethodPost, "/token", bytes.NewBuffer(body))
		rec := httptest.NewRecorder()

		api.generateTokenHandler(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		var response GenerateTokenResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.NotEmpty(t, response.KeyID)
		assert.NotEmpty(t, response.Token)
		assert.Equal(t, 3600, response.TTL)
	})

	t.Run("Giving 0 TTL defaults to TTL of one hour", func(t *testing.T) {
		auth.EXPECT().GenerateToken(mock.Anything, "test-key-id", 0).Return(&token.Token{
			ID:     "test-key-id",
			Secret: "test-token",
			TTL:    time.Hour,
		}, nil).Once()

		requestBody := GenerateTokenRequest{
			KeyID: "test-key-id",
			TTL:   0,
		}
		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest(http.MethodPost, "/token", bytes.NewBuffer(body))
		rec := httptest.NewRecorder()

		api.generateTokenHandler(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)

		var response GenerateTokenResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "test-key-id", response.KeyID)
		assert.NotEmpty(t, response.Token)
		assert.Equal(t, 3600, response.TTL)
	})

	t.Run("Token Generation Error", func(t *testing.T) {
		auth.EXPECT().GenerateToken(mock.Anything, "test-key-id", 3600).Return(nil, errors.New("token generation error")).Once()

		requestBody := GenerateTokenRequest{
			KeyID: "test-key-id",
			TTL:   3600,
		}
		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest(http.MethodPost, "/token", bytes.NewBuffer(body))
		rec := httptest.NewRecorder()

		api.generateTokenHandler(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("Duplicate Token ID Error", func(t *testing.T) {
		auth.EXPECT().GenerateToken(mock.Anything, "test-key-id", 3600).Return(nil, core.ErrDuplicateTokenID).Once()

		requestBody := GenerateTokenRequest{
			KeyID: "test-key-id",
			TTL:   3600,
		}
		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest(http.MethodPost, "/token", bytes.NewBuffer(body))
		rec := httptest.NewRecorder()

		api.generateTokenHandler(rec, req)
		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, "Duplicate token ID\n", rec.Body.String())
	})

	t.Run("JSON Encoding Error", func(_ *testing.T) {
		auth.EXPECT().GenerateToken(mock.Anything, "test-key-id", 3600).Return(&token.Token{
			ID:     "test-key-id",
			Secret: "test-token",
			TTL:    3600,
		}, nil).Once()

		requestBody := GenerateTokenRequest{
			KeyID: "test-key-id",
			TTL:   3600,
		}
		body, _ := json.Marshal(requestBody)
		req := httptest.NewRequest(http.MethodPost, "/token", bytes.NewBuffer(body))
		mockWriter := &mockResponseWriter{ResponseWriter: httptest.NewRecorder()}

		api.generateTokenHandler(mockWriter, req)
	})
}

func TestAPIRun(t *testing.T) {
	svc := NewMockService(t)
	api := New(Config{Listen: ":58083"}, svc)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		err := api.Run(ctx)
		assert.NoError(t, err, "API server should shut down gracefully")
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		assert.Fail(t, "API server did not shut down in time")
	}
}

func TestRevokeTokenHandler(t *testing.T) {
	auth := NewMockService(t)
	api := New(Config{}, auth)

	tests := []struct {
		mockBehavior func()
		name         string
		keyID        string
		expectedBody string
		expectedCode int
	}{
		{
			name:         "Missing KeyID",
			keyID:        "",
			mockBehavior: func() {},
			expectedCode: http.StatusBadRequest,
			expectedBody: "Key ID is required\n",
		},
		{
			name:  "Successful Revocation",
			keyID: "test-key-id",
			mockBehavior: func() {
				auth.EXPECT().DeleteToken(mock.Anything, "test-key-id").Return(nil).Once()
			},
			expectedCode: http.StatusNoContent,
			expectedBody: "",
		},
		{
			name:  "Token Not Found",
			keyID: "test-key-id",
			mockBehavior: func() {
				auth.EXPECT().DeleteToken(mock.Anything, "test-key-id").Return(core.ErrTokenNotFound).Once()
			},
			expectedCode: http.StatusNotFound,
			expectedBody: "Token not found\n",
		},
		{
			name:  "Internal Error",
			keyID: "test-key-id",
			mockBehavior: func() {
				auth.EXPECT().DeleteToken(mock.Anything, "test-key-id").Return(errors.New("failed to delete token")).Once()
			},
			expectedCode: http.StatusInternalServerError,
			expectedBody: "Internal Server Error\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockBehavior()

			req := httptest.NewRequest(http.MethodDelete, "/token/"+tt.keyID, http.NoBody)

			if tt.keyID != "" {
				req.SetPathValue("keyID", tt.keyID)
			}

			rec := httptest.NewRecorder()

			api.RevokeTokenHandler(rec, req)

			assert.Equal(t, tt.expectedCode, rec.Code)
			assert.Equal(t, tt.expectedBody, rec.Body.String())
		})
	}
}
