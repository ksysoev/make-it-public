package dummy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
	}{
		{
			name:        "Valid status code",
			config:      Config{Status: 200},
			expectError: false,
		},
		{
			name:        "Invalid low status code",
			config:      Config{Status: 199},
			expectError: true,
		},
		{
			name:        "Invalid high status code",
			config:      Config{Status: 600},
			expectError: true,
		},
		{
			name:        "Valid status with body",
			config:      Config{Status: 201, Body: "Created"},
			expectError: false,
		},
		{
			name:        "Valid status with JSON",
			config:      Config{Status: 200, JSON: `{"message": "success"}`},
			expectError: false,
		},
		{
			name:        "Body and JSON together",
			config:      Config{Status: 200, Body: "Conflict", JSON: `{"message": "error"}`},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := New(tt.config)

			if tt.expectError {
				assert.Error(t, err, "Expected an error for invalid config")
				assert.Nil(t, server, "Server should be nil on error")
			} else {
				require.NoError(t, err, "Failed to create server")
				assert.NotNil(t, server, "Server should not be nil")
				assert.NotNil(t, server.isReady, "isReady channel should not be nil")
				assert.NotNil(t, server.jsonFmt, "jsonFmt should not be nil")
				assert.Empty(t, server.addr, "addr should be empty initially")
			}
		})
	}
}

func TestRun(t *testing.T) {
	server, err := New(Config{Status: 200, Body: "ok"})

	require.NoError(t, err, "Failed to create server")

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Run(ctx)
	}()

	// Wait for the server to be ready
	addr := server.Addr()
	assert.NotEmpty(t, addr, "Server address should not be empty")

	// Make a request to the server to verify it's running
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr, http.NoBody)
	require.NoError(t, err)

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)

	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "ok", string(body))

	// Cancel the context to stop the server
	cancel()

	// Check if the server stopped without error
	select {
	case err := <-errCh:
		assert.Error(t, err) // We expect an error when the server is closed
	case <-time.After(2 * time.Second):
		t.Fatal("Server did not stop within timeout")
	}
}

func TestAddr(t *testing.T) {
	server, err := New(Config{Status: 200})

	require.NoError(t, err, "Failed to create server")

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the server in a goroutine
	go func() {
		_ = server.Run(ctx)
	}()

	// Get the address
	addr := server.Addr()
	assert.NotEmpty(t, addr, "Server address should not be empty")

	// Verify that the address is in the correct format (host:port)
	parts := strings.Split(addr, ":")
	assert.Equal(t, 2, len(parts), "Address should be in host:port format")
	assert.Equal(t, "127.0.0.1", parts[0], "Host should be 127.0.0.1")

	// Cancel the context to stop the server
	cancel()
}

func TestServeHTTP(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		url         string
		body        string
		contentType string
	}{
		{
			name:        "GET request without body",
			method:      http.MethodGet,
			url:         "/test",
			body:        "",
			contentType: "",
		},
		{
			name:        "POST request with JSON body",
			method:      http.MethodPost,
			url:         "/api/data",
			body:        `{"key": "value"}`,
			contentType: "application/json",
		},
		{
			name:        "PUT request with text body",
			method:      http.MethodPut,
			url:         "/update",
			body:        "Hello, world!",
			contentType: "text/plain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := New(Config{Status: 200, Body: "ok"})

			require.NoError(t, err, "Failed to create server")

			// Create a request
			var bodyReader io.Reader
			if tt.body != "" {
				bodyReader = strings.NewReader(tt.body)
			}

			req := httptest.NewRequest(tt.method, tt.url, bodyReader)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			// Create a response recorder
			w := httptest.NewRecorder()

			// Temporarily redirect stdout to capture output
			oldStdout := os.Stdout
			r, w2, _ := os.Pipe()
			os.Stdout = w2

			// Call ServeHTTP
			server.ServeHTTP(w, req)

			// Restore stdout
			require.NoError(t, w2.Close())

			os.Stdout = oldStdout

			// Read captured output
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)
			output := buf.String()

			// Verify response
			resp := w.Result()
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, http.StatusOK, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, "ok", string(body))

			// Verify that the output contains the request details
			assert.Contains(t, output, tt.method)
			assert.Contains(t, output, tt.url)

			if tt.body != "" && tt.contentType != "" {
				switch tt.contentType {
				case "application/json":
					assert.Contains(t, output, "key")
					assert.Contains(t, output, "value")
				case "text/plain":
					assert.Contains(t, output, tt.body)
				}
			}
		})
	}
}

func TestPrintBody(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		data        []byte
		expectError bool
	}{
		{
			name:        "JSON content type",
			data:        []byte(`{"key": "value"}`),
			contentType: "application/json",
			expectError: false,
		},
		{
			name:        "Text content type",
			data:        []byte("Hello, world!"),
			contentType: "text/plain",
			expectError: false,
		},
		{
			name:        "Unsupported content type",
			data:        []byte{0x01, 0x02, 0x03},
			contentType: "application/octet-stream",
			expectError: true,
		},
		{
			name:        "JSON with parameters",
			data:        []byte(`{"key": "value"}`),
			contentType: "application/json; charset=utf-8",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := New(Config{Status: 200})

			require.NoError(t, err, "Failed to create server")

			// Temporarily redirect stdout to capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Call printBody
			err = server.printBody(tt.data, tt.contentType)

			// Restore stdout
			require.NoError(t, w.Close())

			os.Stdout = oldStdout

			// Read captured output
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)
			output := buf.String()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				if strings.HasPrefix(tt.contentType, "application/json") {
					// For JSON, verify that the output contains the key and value
					assert.Contains(t, output, "key")
					assert.Contains(t, output, "value")
				} else if strings.HasPrefix(tt.contentType, "text/") {
					// For text, verify that the output contains the text
					assert.Contains(t, output, string(tt.data))
				}
			}
		})
	}
}

func TestPrintText(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{
			name: "Simple text",
			data: []byte("Hello, world!"),
		},
		{
			name: "Empty text",
			data: []byte(""),
		},
		{
			name: "Multi-line text",
			data: []byte("Line 1\nLine 2\nLine 3"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := New(Config{Status: 200})

			require.NoError(t, err, "Failed to create server")

			// Temporarily redirect stdout to capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Call printText
			err = server.printText(tt.data)

			// Restore stdout
			require.NoError(t, w.Close())

			os.Stdout = oldStdout

			// Read captured output
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)
			output := buf.String()

			assert.NoError(t, err)
			assert.Contains(t, output, string(tt.data))
		})
	}
}

func TestPrintJSON(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expectError bool
	}{
		{
			name:        "Valid JSON object",
			data:        []byte(`{"key": "value", "number": 42, "bool": true, "null": null}`),
			expectError: false,
		},
		{
			name:        "Valid JSON array",
			data:        []byte(`[1, 2, 3, 4, 5]`),
			expectError: false,
		},
		{
			name:        "Invalid JSON",
			data:        []byte(`{"key": "value"`), // Missing closing brace
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := New(Config{Status: 200})

			require.NoError(t, err, "Failed to create server")

			// Temporarily redirect stdout to capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Call printJSON
			err = server.printJSON(tt.data)

			// Restore stdout
			require.NoError(t, w.Close())

			os.Stdout = oldStdout

			// Read captured output
			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)
			output := buf.String()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Parse the original JSON to verify it's in the output
				var originalData interface{}
				err := json.Unmarshal(tt.data, &originalData)
				require.NoError(t, err)

				// Check for key elements in the output
				switch v := originalData.(type) {
				case map[string]interface{}:
					for key := range v {
						assert.Contains(t, output, key)
					}
				case []interface{}:
					for _, item := range v {
						assert.Contains(t, output, fmt.Sprintf("%v", item))
					}
				}
			}
		})
	}
}

func TestPrintHeaders(t *testing.T) {
	tests := []struct {
		headers http.Header
		name    string
	}{
		{
			name: "Standard headers",
			headers: http.Header{
				"Content-Type":    []string{"application/json"},
				"Content-Length":  []string{"42"},
				"Accept-Encoding": []string{"gzip", "deflate"},
				"User-Agent":      []string{"test-agent"},
			},
		},
		{
			name:    "Empty headers",
			headers: http.Header{},
		},
		{
			name: "Headers with multiple values",
			headers: http.Header{
				"Accept": []string{"text/html", "application/xhtml+xml", "application/xml"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a buffer to capture the output
			var buf bytes.Buffer

			// Call printHeaders
			printHeaders(tt.headers, &buf)

			// Get the output
			output := buf.String()

			// Verify that all headers are in the output
			for header, values := range tt.headers {
				for _, value := range values {
					assert.Contains(t, output, header+": "+value)
				}
			}

			// Verify that headers are sorted
			if len(tt.headers) > 1 {
				headerNames := make([]string, 0, len(tt.headers))
				for header := range tt.headers {
					headerNames = append(headerNames, header)
				}

				// Sort the header names
				sortedHeaderNames := make([]string, len(headerNames))
				copy(sortedHeaderNames, headerNames)
				sort.Strings(sortedHeaderNames)

				// Check if the output follows the sorted order
				lastIndex := -1

				for _, header := range sortedHeaderNames {
					index := strings.Index(output, header+":")
					assert.True(t, index > lastIndex, "Headers should be sorted")
					lastIndex = index
				}
			}
		})
	}
}
