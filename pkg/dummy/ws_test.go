package dummy

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWSEchoServer(t *testing.T) {
	tests := []struct {
		name   string
		config WSConfig
	}{
		{
			name:   "Interactive mode",
			config: WSConfig{Interactive: true},
		},
		{
			name:   "Non-interactive mode",
			config: WSConfig{Interactive: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewWSEchoServer(tt.config)

			require.NoError(t, err, "Failed to create WebSocket echo server")
			assert.NotNil(t, server, "Server should not be nil")
			assert.NotNil(t, server.isReady, "isReady channel should not be nil")
			assert.Empty(t, server.addr, "addr should be empty initially")
			assert.Equal(t, tt.config.Interactive, server.interactive, "interactive flag should match config")
		})
	}
}

func TestWSEchoServerRun(t *testing.T) {
	server, err := NewWSEchoServer(WSConfig{Interactive: false})
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

	// Connect to the WebSocket server
	wsURL := "ws://" + addr
	conn, _, err := websocket.Dial(ctx, wsURL, nil) //nolint:bodyclose // WebSocket connections don't have response bodies to close
	require.NoError(t, err, "Failed to connect to WebSocket server")

	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	// Verify connection is established
	assert.NotNil(t, conn, "WebSocket connection should not be nil")

	// Cancel the context to stop the server
	cancel()

	// Check if the server stopped
	select {
	case err := <-errCh:
		assert.Error(t, err) // We expect an error when the server is closed
	case <-time.After(2 * time.Second):
		t.Fatal("Server did not stop within timeout")
	}
}

func TestWSEchoServerAddr(t *testing.T) {
	server, err := NewWSEchoServer(WSConfig{Interactive: false})
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

func TestWSEchoServerEchoText(t *testing.T) {
	server, err := NewWSEchoServer(WSConfig{Interactive: false})
	require.NoError(t, err, "Failed to create server")

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the server in a goroutine
	go func() {
		_ = server.Run(ctx)
	}()

	// Wait for the server to be ready
	addr := server.Addr()

	// Connect to the WebSocket server
	wsURL := "ws://" + addr
	conn, _, err := websocket.Dial(ctx, wsURL, nil) //nolint:bodyclose // WebSocket connections don't have response bodies to close
	require.NoError(t, err, "Failed to connect to WebSocket server")

	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	// Send a text message
	testMessage := "Hello, WebSocket!"
	err = conn.Write(ctx, websocket.MessageText, []byte(testMessage))
	require.NoError(t, err, "Failed to send message")

	// Read the echoed message
	msgType, data, err := conn.Read(ctx)
	require.NoError(t, err, "Failed to read message")

	// Verify the echo
	assert.Equal(t, websocket.MessageText, msgType, "Message type should be text")
	assert.Equal(t, testMessage, string(data), "Echoed message should match sent message")
}

func TestWSEchoServerEchoBinary(t *testing.T) {
	server, err := NewWSEchoServer(WSConfig{Interactive: false})
	require.NoError(t, err, "Failed to create server")

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the server in a goroutine
	go func() {
		_ = server.Run(ctx)
	}()

	// Wait for the server to be ready
	addr := server.Addr()

	// Connect to the WebSocket server
	wsURL := "ws://" + addr
	conn, _, err := websocket.Dial(ctx, wsURL, nil) //nolint:bodyclose // WebSocket connections don't have response bodies to close
	require.NoError(t, err, "Failed to connect to WebSocket server")

	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	// Send a binary message
	testData := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	err = conn.Write(ctx, websocket.MessageBinary, testData)
	require.NoError(t, err, "Failed to send binary message")

	// Read the echoed message
	msgType, data, err := conn.Read(ctx)
	require.NoError(t, err, "Failed to read message")

	// Verify the echo
	assert.Equal(t, websocket.MessageBinary, msgType, "Message type should be binary")
	assert.Equal(t, testData, data, "Echoed binary data should match sent data")
}

func TestWSEchoServerMultipleMessages(t *testing.T) {
	server, err := NewWSEchoServer(WSConfig{Interactive: false})
	require.NoError(t, err, "Failed to create server")

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the server in a goroutine
	go func() {
		_ = server.Run(ctx)
	}()

	// Wait for the server to be ready
	addr := server.Addr()

	// Connect to the WebSocket server
	wsURL := "ws://" + addr
	conn, _, err := websocket.Dial(ctx, wsURL, nil) //nolint:bodyclose // WebSocket connections don't have response bodies to close
	require.NoError(t, err, "Failed to connect to WebSocket server")

	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	// Send and verify multiple messages
	testMessages := []string{
		"First message",
		"Second message",
		"Third message",
	}

	for _, msg := range testMessages {
		// Send message
		err = conn.Write(ctx, websocket.MessageText, []byte(msg))
		require.NoError(t, err, "Failed to send message: %s", msg)

		// Read echo
		msgType, data, err := conn.Read(ctx)
		require.NoError(t, err, "Failed to read message")

		// Verify
		assert.Equal(t, websocket.MessageText, msgType, "Message type should be text")
		assert.Equal(t, msg, string(data), "Echoed message should match sent message")
	}
}

func TestWSEchoServerLargeBinaryMessage(t *testing.T) {
	server, err := NewWSEchoServer(WSConfig{Interactive: false})
	require.NoError(t, err, "Failed to create server")

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the server in a goroutine
	go func() {
		_ = server.Run(ctx)
	}()

	// Wait for the server to be ready
	addr := server.Addr()

	// Connect to the WebSocket server
	wsURL := "ws://" + addr
	conn, _, err := websocket.Dial(ctx, wsURL, nil) //nolint:bodyclose // WebSocket connections don't have response bodies to close
	require.NoError(t, err, "Failed to connect to WebSocket server")

	// Set read limit on client side as well
	conn.SetReadLimit(10 * 1024 * 1024)

	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	// Create a large binary message (1 MB)
	testData := make([]byte, 1024*1024)
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	// Send the large message
	err = conn.Write(ctx, websocket.MessageBinary, testData)
	require.NoError(t, err, "Failed to send large binary message")

	// Read the echoed message
	msgType, data, err := conn.Read(ctx)
	require.NoError(t, err, "Failed to read message")

	// Verify the echo
	assert.Equal(t, websocket.MessageBinary, msgType, "Message type should be binary")
	assert.Equal(t, len(testData), len(data), "Echoed data length should match")
	assert.Equal(t, testData, data, "Echoed binary data should match sent data")
}
