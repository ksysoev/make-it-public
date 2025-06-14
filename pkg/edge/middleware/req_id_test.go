package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReqID_BasicRequest(t *testing.T) {
	var capturedReqID string

	handler := ReqID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Context().Value("req_id")
		require.NotNil(t, reqID)

		reqIDStr, ok := reqID.(string)
		require.True(t, ok)
		require.NotEmpty(t, reqIDStr)

		_, err := uuid.Parse(reqIDStr)
		require.NoError(t, err)

		capturedReqID = reqIDStr

		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	resp := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(resp, req)

	// Assert
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.NotEmpty(t, capturedReqID)
}
