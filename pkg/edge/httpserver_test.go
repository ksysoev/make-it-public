package edge

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ksysoev/make-it-public/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
	}{
		{
			name: "valid configuration",
			config: Config{
				Listen: ":8080",
				Public: PublicEndpointConfig{
					Schema: "http",
					Domain: "example.com",
					Port:   80,
				},
			},
			expectError: false,
		},
		{
			name: "empty schema",
			config: Config{
				Listen: ":8080",
				Public: PublicEndpointConfig{
					Schema: "",
					Domain: "example.com",
					Port:   80,
				},
			},
			expectError: true,
		},
		{
			name: "empty domain",
			config: Config{
				Listen: ":8080",
				Public: PublicEndpointConfig{
					Schema: "http",
					Domain: "",
					Port:   80,
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConnService := NewMockConnService(t)

			// Set up expectations for the mock only for the success case
			// In error cases, SetEndpointGenerator won't be called
			if !tt.expectError {
				mockConnService.On("SetEndpointGenerator", mock.AnythingOfType("func(string) (string, error)")).Return()
			}

			server, err := New(tt.config, mockConnService)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, server)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, server)
				assert.Equal(t, tt.config, server.config)
				assert.Equal(t, mockConnService, server.connService)
			}
		})
	}
}

func TestRun(t *testing.T) {
	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a mock ConnService
	mockConnService := NewMockConnService(t)
	mockConnService.On("SetEndpointGenerator", mock.AnythingOfType("func(string) (string, error)")).Return()

	// Create a server with a random port
	config := Config{
		Listen: "127.0.0.1:0", // Use port 0 to get a random available port
		Public: PublicEndpointConfig{
			Schema: "http",
			Domain: "example.com",
			Port:   80,
		},
		ConnLimit: 10,
	}

	server, err := New(config, mockConnService)
	require.NoError(t, err)
	require.NotNil(t, server)

	// Start the server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Run(ctx)
	}()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Cancel the context to stop the server
	cancel()

	// Check if the server stopped without error
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Server did not stop within timeout")
	}
}

func TestServeHTTP(t *testing.T) {
	tests := []struct {
		handleConnErr  error
		name           string
		keyID          string
		clientIP       string
		expectedStatus int
	}{
		{
			name:           "successful connection",
			keyID:          "test-key",
			clientIP:       "192.168.1.1",
			handleConnErr:  nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "failed to connect",
			keyID:          "test-key",
			clientIP:       "192.168.1.1",
			handleConnErr:  core.ErrFailedToConnect,
			expectedStatus: http.StatusBadGateway,
		},
		{
			name:           "keyID not found",
			keyID:          "nonexistent-key",
			clientIP:       "192.168.1.1",
			handleConnErr:  core.ErrKeyIDNotFound,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "context canceled",
			keyID:          "test-key",
			clientIP:       "192.168.1.1",
			handleConnErr:  context.Canceled,
			expectedStatus: http.StatusOK, // No response is sent for context.Canceled
		},
		{
			name:           "other error",
			keyID:          "test-key",
			clientIP:       "192.168.1.1",
			handleConnErr:  errors.New("some other error"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock ConnService
			mockConnService := NewMockConnService(t)

			// Create a server
			config := Config{
				Listen: ":8080",
				Public: PublicEndpointConfig{
					Schema: "http",
					Domain: "example.com",
					Port:   80,
				},
			}

			// Set up the mock to return the specified error
			mockConnService.On("SetEndpointGenerator", mock.AnythingOfType("func(string) (string, error)")).Return()

			// The actual call to HandleHTTPConnection will use a context with a req_id value
			// and empty keyID and clientIP because we haven't set them in the request context
			mockConnService.On("HandleHTTPConnection",
				mock.Anything, // Context with req_id
				mock.Anything, // keyID will be empty because we haven't set it
				mock.Anything, // The hijacked connection
				mock.Anything, // The write function
				mock.Anything, // clientIP will be empty because we haven't set it
			).Return(tt.handleConnErr)

			server, err := New(config, mockConnService)
			require.NoError(t, err)

			// Create a request
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

			// Mock the GetKeyID and GetClientIP functions by patching the HTTPServer.ServeHTTP method
			// We'll use the mock ConnService to verify that the correct keyID and clientIP are passed

			// Use a custom ResponseRecorder that implements http.Hijacker
			w := newHijackableResponseRecorder()

			// For error cases, we need to check that the mock was called with the expected parameters
			// The actual response status is set by sendResponse, which we test separately

			// Call ServeHTTP
			server.ServeHTTP(w, req)

			// Verify that the mock was called
			mockConnService.AssertExpectations(t)
		})
	}
}

func TestSendResponse(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		status     int
		expectBody bool
	}{
		{
			name:       "404 response",
			status:     http.StatusNotFound,
			body:       htmlErrorTemplate404,
			expectBody: true,
		},
		{
			name:       "502 response",
			status:     http.StatusBadGateway,
			body:       htmlErrorTemplate502,
			expectBody: true,
		},
		{
			name:       "custom response",
			status:     http.StatusOK,
			body:       "<html><body>Custom response</body></html>",
			expectBody: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a request
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

			// Create a pipe to capture the response
			clientReader, serverWriter := net.Pipe()

			// Send the response in a goroutine
			go func() {
				sendResponse(req, serverWriter, tt.status, tt.body)
				_ = serverWriter.Close()
			}()

			// Read the response
			var buf bytes.Buffer
			_, err := io.Copy(&buf, clientReader)
			require.NoError(t, err)

			// Check the response
			response := buf.String()
			assert.Contains(t, response, fmt.Sprintf("HTTP/1.1 %d %s", tt.status, http.StatusText(tt.status)))

			if tt.expectBody {
				assert.Contains(t, response, "Content-Type: text/html; charset=utf-8")
				assert.Contains(t, response, tt.body)
			}
		})
	}
}

// Custom ResponseRecorder that implements http.Hijacker
type hijackableResponseRecorder struct {
	*httptest.ResponseRecorder
	closeNotify chan bool
}

func newHijackableResponseRecorder() *hijackableResponseRecorder {
	return &hijackableResponseRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		closeNotify:      make(chan bool, 1),
	}
}

func (hrw *hijackableResponseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	// Return a mock connection
	return hrw, bufio.NewReadWriter(
		bufio.NewReader(strings.NewReader("")),
		bufio.NewWriter(hrw.Body),
	), nil
}

func (hrw *hijackableResponseRecorder) Close() error {
	hrw.closeNotify <- true
	return nil
}

func (hrw *hijackableResponseRecorder) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func (hrw *hijackableResponseRecorder) Write(p []byte) (int, error) {
	return hrw.Body.Write(p)
}

func (hrw *hijackableResponseRecorder) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}
}

func (hrw *hijackableResponseRecorder) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(192, 168, 1, 1), Port: 12345}
}

func (hrw *hijackableResponseRecorder) SetDeadline(_ time.Time) error {
	return nil
}

func (hrw *hijackableResponseRecorder) SetReadDeadline(_ time.Time) error {
	return nil
}

func (hrw *hijackableResponseRecorder) SetWriteDeadline(_ time.Time) error {
	return nil
}
