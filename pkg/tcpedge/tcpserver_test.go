package tcpedge

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// findFreePort asks the OS for an available TCP port and returns it.
// The port is briefly bound then released; there is a small TOCTOU window,
// but it is far safer than hard-coding a port that may be in use on CI.
func findFreePort(t *testing.T) int {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	addr, ok := l.Addr().(*net.TCPAddr)
	require.True(t, ok, "expected TCP address")

	port := addr.Port

	require.NoError(t, l.Close())

	return port
}

// validConfig returns a Config that passes Validate().
func validConfig(t *testing.T) Config {
	t.Helper()

	base := findFreePort(t)

	return Config{
		ListenHost: "127.0.0.1",
		Public:     PublicConfig{Host: "example.com"},
		PortRange:  PortRange{Min: base, Max: base + 100},
	}
}

func TestNew_InvalidConfig(t *testing.T) {
	svc := NewMockConnService(t)

	cfg := Config{} // missing required fields

	_, err := New(cfg, svc)
	assert.Error(t, err)
}

func TestNew_InjectsAllocator(t *testing.T) {
	svc := NewMockConnService(t)
	svc.EXPECT().SetTCPEndpointAllocator(mock.Anything)

	_, err := New(validConfig(t), svc)
	require.NoError(t, err)
}

func TestTCPServer_Run_Shutdown(t *testing.T) {
	svc := NewMockConnService(t)
	svc.EXPECT().SetTCPEndpointAllocator(mock.Anything)

	srv, err := New(validConfig(t), svc)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)

	go func() { done <- srv.Run(ctx) }()

	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}
}

func TestTCPServer_Allocate_ReturnsEndpoint(t *testing.T) {
	svc := NewMockConnService(t)
	svc.EXPECT().SetTCPEndpointAllocator(mock.Anything)

	srv, err := New(validConfig(t), svc)
	require.NoError(t, err)

	defer srv.closeAllListeners()

	endpoint, err := srv.Allocate(context.Background(), "testkey")
	require.NoError(t, err)

	// Endpoint should be "example.com:<port>".
	host, portStr, splitErr := net.SplitHostPort(endpoint)
	require.NoError(t, splitErr)
	assert.Equal(t, "example.com", host)
	assert.NotEmpty(t, portStr)
}

func TestTCPServer_Allocate_DuplicateKeyID(t *testing.T) {
	svc := NewMockConnService(t)
	svc.EXPECT().SetTCPEndpointAllocator(mock.Anything)

	srv, err := New(validConfig(t), svc)
	require.NoError(t, err)

	defer srv.closeAllListeners()

	_, err = srv.Allocate(context.Background(), "dup")
	require.NoError(t, err)

	_, err = srv.Allocate(context.Background(), "dup")
	assert.ErrorIs(t, err, ErrKeyIDAlreadyAllocated)
}

func TestTCPServer_Release_FreesPort(t *testing.T) {
	svc := NewMockConnService(t)
	svc.EXPECT().SetTCPEndpointAllocator(mock.Anything)

	srv, err := New(validConfig(t), svc)
	require.NoError(t, err)

	initial := srv.portPool.Available()

	_, err = srv.Allocate(context.Background(), "releasekey")
	require.NoError(t, err)

	assert.Equal(t, initial-1, srv.portPool.Available())

	srv.Release("releasekey")
	assert.Equal(t, initial, srv.portPool.Available())
}

func TestTCPServer_Release_Idempotent(t *testing.T) {
	svc := NewMockConnService(t)
	svc.EXPECT().SetTCPEndpointAllocator(mock.Anything)

	srv, err := New(validConfig(t), svc)
	require.NoError(t, err)

	// Releasing a key that was never allocated should not panic.
	assert.NotPanics(t, func() { srv.Release("nonexistent") })

	_, err = srv.Allocate(context.Background(), "idem")
	require.NoError(t, err)

	srv.Release("idem")

	// Second release should also be a no-op.
	assert.NotPanics(t, func() { srv.Release("idem") })
}

func TestTCPServer_AcceptsAndRoutesConnection(t *testing.T) {
	svc := NewMockConnService(t)
	svc.EXPECT().SetTCPEndpointAllocator(mock.Anything)

	connReceived := make(chan struct{})

	svc.EXPECT().
		HandleTCPConnection(mock.Anything, "routekey", mock.Anything, mock.MatchedBy(func(ip string) bool {
			return ip == "127.0.0.1"
		})).
		RunAndReturn(func(_ context.Context, _ string, conn net.Conn, _ string) error {
			close(connReceived)
			return nil
		})

	srv, err := New(validConfig(t), svc)
	require.NoError(t, err)

	defer srv.closeAllListeners()

	endpoint, err := srv.Allocate(context.Background(), "routekey")
	require.NoError(t, err)

	// endpoint is "example.com:<port>" — extract the port and dial the listen address.
	_, portStr, err := net.SplitHostPort(endpoint)
	require.NoError(t, err)

	dialAddr := net.JoinHostPort("127.0.0.1", portStr)

	// Dial the allocated port as if we were an end-user.
	c, dialErr := net.Dial("tcp", dialAddr)
	require.NoError(t, dialErr)

	defer c.Close()

	select {
	case <-connReceived:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("HandleTCPConnection was not called within timeout")
	}
}

func TestTCPServer_PortExhausted(t *testing.T) {
	// Use a range of exactly 1 port so exhaustion happens on the second Allocate.
	// min == max is a valid single-port range.
	port := findFreePort(t)
	cfg := Config{
		ListenHost: "127.0.0.1",
		Public:     PublicConfig{Host: "example.com"},
		PortRange:  PortRange{Min: port, Max: port},
	}

	svc := NewMockConnService(t)
	svc.EXPECT().SetTCPEndpointAllocator(mock.Anything)

	srv, err := New(cfg, svc)
	require.NoError(t, err)

	defer srv.closeAllListeners()

	_, err = srv.Allocate(context.Background(), "key1")
	require.NoError(t, err)

	_, err = srv.Allocate(context.Background(), "key2")
	assert.Error(t, err, "should fail when port pool is exhausted")
	assert.ErrorIs(t, err, ErrPortPoolExhausted)
}

func TestTCPServer_CloseAllListeners(t *testing.T) {
	svc := NewMockConnService(t)
	svc.EXPECT().SetTCPEndpointAllocator(mock.Anything)

	srv, err := New(validConfig(t), svc)
	require.NoError(t, err)

	for i := range 3 {
		_, allocErr := srv.Allocate(context.Background(), fmt.Sprintf("key%d", i))
		require.NoError(t, allocErr)
	}

	srv.closeAllListeners()

	// After closeAll, all listeners should be gone.
	srv.mu.RLock()
	remaining := len(srv.listeners)
	srv.mu.RUnlock()

	assert.Zero(t, remaining)

	// All ports should be returned to the pool.
	cfg := validConfig(t)
	expected := cfg.PortRange.Max - cfg.PortRange.Min + 1
	assert.Equal(t, expected, srv.portPool.Available())
}
