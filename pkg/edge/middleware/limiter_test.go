package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLimiter_Allow(t *testing.T) {
	tests := []struct {
		name      string
		limit     int
		key       string
		repeat    int
		allowAll  bool
		finalFail bool
	}{
		{"within limit", 3, "test-key", 2, true, false},
		{"exceeds limit", 2, "test-key", 3, false, true},
		{"exact limit", 1, "test-key", 1, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := newLimiter(tt.limit)

			success := true
			for i := 0; i < tt.repeat; i++ {
				success = l.Allow(tt.key)
			}

			assert.Equal(t, tt.allowAll, success)
			assert.Equal(t, tt.finalFail, !l.Allow(tt.key))
		})
	}
}

func TestLimiter_Release(t *testing.T) {
	l := newLimiter(2)
	l.Allow("test-key")
	l.Allow("test-key")

	l.Release("test-key")
	assert.Equal(t, 1, l.counter["test-key"])

	l.Release("test-key")
	n, exists := l.counter["test-key"]
	assert.Equal(t, 0, n)
	assert.False(t, exists)
}

func TestNewLimiter(t *testing.T) {
	limit := 5
	l := newLimiter(limit)

	assert.NotNil(t, l)
	assert.Equal(t, limit, l.limit)
	assert.NotNil(t, l.counter)
}

func TestLimitConnections(t *testing.T) {
	tests := []struct {
		name           string
		maxConnections int
		requests       int
		expectTooMany  bool
	}{
		{"within limit", 2, 2, false},
		{"exceeds limit", 2, 3, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := make(chan struct{})
			done := make(chan bool, 3)
			mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				<-start
				time.Sleep(10 * time.Millisecond)
				w.WriteHeader(http.StatusOK)
			})

			limiterMiddleware := LimitConnections(tt.maxConnections)(mockHandler)

			for i := 0; i < tt.requests; i++ {
				go func() {
					req := httptest.NewRequest(http.MethodGet, "/", nil)
					recorder := httptest.NewRecorder()
					limiterMiddleware.ServeHTTP(recorder, req)

					if http.StatusTooManyRequests == recorder.Code {
						done <- false
					} else {
						done <- true
					}
				}()
			}

			close(start)

			countErrors := 0
			for i := 0; i < tt.requests; i++ {
				select {
				case result := <-done:
					if !result {
						countErrors++
					}
				case <-time.After(100 * time.Millisecond):
					t.Error("timeout waiting for request")
				}
			}

			if tt.expectTooMany {
				assert.Equal(t, 1, countErrors)
			} else {
				assert.Equal(t, 0, countErrors)
			}
		})
	}
}
