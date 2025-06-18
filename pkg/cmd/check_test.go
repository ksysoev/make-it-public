package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunHealthCheck_Success(t *testing.T) {
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("healthy"))
	}))

	defer httpServer.Close()

	t.Setenv("API_LISTEN", httpServer.Listener.Addr().String())

	err := RunHealthCheck(t.Context(), &args{LogLevel: "error"})
	assert.NoError(t, err)
}

func TestRunHealthCheck_Failure(t *testing.T) {
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	defer httpServer.Close()

	t.Setenv("API_LISTEN", httpServer.Listener.Addr().String())

	err := RunHealthCheck(t.Context(), &args{LogLevel: "error"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed")
}
