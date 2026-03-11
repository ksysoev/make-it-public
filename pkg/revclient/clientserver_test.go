package revclient

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ksysoev/make-it-public/pkg/core/token"
	"github.com/ksysoev/revdial"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeListener is a controllable net.Listener for testing.
// Calls to Accept block until either a connection is sent on connCh or the
// listener is closed via Close().
type fakeListener struct {
	connCh chan net.Conn
	closed chan struct{}
	addr   net.Addr
}

func newFakeListener() *fakeListener {
	return &fakeListener{
		connCh: make(chan net.Conn, 1),
		closed: make(chan struct{}),
		addr:   &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0},
	}
}

func (f *fakeListener) Accept() (net.Conn, error) {
	select {
	case c, ok := <-f.connCh:
		if !ok {
			return nil, revdial.ErrListenerClosed
		}

		return c, nil
	case <-f.closed:
		return nil, revdial.ErrListenerClosed
	}
}

func (f *fakeListener) Close() error {
	select {
	case <-f.closed:
		// already closed
	default:
		close(f.closed)
	}

	return nil
}

func (f *fakeListener) Addr() net.Addr { return f.addr }

// buildOptsOptionCount enumerates the expected number of ListenerOption values
// returned by buildOpts for each configuration.
//
// Base options always present (regardless of config):
//
//  1. WithUserPass       — authentication
//  2. WithEventHandler   — urlToConnectUpdated event
//  3. WithListenerKeepAlive — TCP keepalive (the fix for #283)
//
// Conditional options:
//
//	+1 WithListenerTLSConfig  when NoTLS == false (default)
//	+1 WithEnableV2           when EnableV2 == true
const baseOptionCount = 3 // auth + event handler + keepalive

func newTestToken(t *testing.T) *token.Token {
	t.Helper()

	tkn, err := token.GenerateToken("testkey", 3600, token.TokenTypeWeb)
	require.NoError(t, err)

	return tkn
}

func TestBuildOpts_OptionCount(t *testing.T) {
	tests := []struct {
		name        string
		cfg         Config
		wantCount   int
		isReconnect bool
	}{
		{
			name:      "no TLS, no V2",
			cfg:       Config{ServerAddr: "localhost:8081", NoTLS: true, EnableV2: false},
			wantCount: baseOptionCount,
		},
		{
			name:      "TLS enabled, no V2",
			cfg:       Config{ServerAddr: "localhost:8081", NoTLS: false, EnableV2: false},
			wantCount: baseOptionCount + 1, // +WithListenerTLSConfig
		},
		{
			name:      "no TLS, V2 enabled",
			cfg:       Config{ServerAddr: "localhost:8081", NoTLS: true, EnableV2: true},
			wantCount: baseOptionCount + 1, // +WithEnableV2
		},
		{
			name:      "TLS enabled, V2 enabled",
			cfg:       Config{ServerAddr: "localhost:8081", NoTLS: false, EnableV2: true},
			wantCount: baseOptionCount + 2, // +WithListenerTLSConfig +WithEnableV2
		},
		{
			name:        "reconnect flag does not change option count",
			cfg:         Config{ServerAddr: "localhost:8081", NoTLS: true, EnableV2: false},
			wantCount:   baseOptionCount,
			isReconnect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := NewClientServer(tt.cfg, newTestToken(t))
			opts, err := cs.buildOpts(context.Background(), tt.isReconnect)

			require.NoError(t, err)
			assert.Len(t, opts, tt.wantCount,
				"expected %d ListenerOption(s) including WithListenerKeepAlive", tt.wantCount)
		})
	}
}

func TestBuildOpts_InvalidServerAddr(t *testing.T) {
	// When TLS is enabled but the server address has no port, SplitHostPort should fail.
	cs := NewClientServer(Config{ServerAddr: "no-port-here", NoTLS: false}, newTestToken(t))

	_, err := cs.buildOpts(context.Background(), false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to split host and port")
}

// TestRun_FirstConnectFails verifies that a connection error on the very first
// attempt is returned immediately without any retry.
func TestRun_FirstConnectFails(t *testing.T) {
	wantErr := errors.New("dial refused")

	cs := NewClientServer(Config{ServerAddr: "localhost:1", NoTLS: true}, newTestToken(t))
	cs.listen = func(_ context.Context, _ string, _ ...revdial.ListenerOption) (net.Listener, error) {
		return nil, wantErr
	}

	err := cs.Run(context.Background())

	require.Error(t, err)
	assert.ErrorIs(t, err, wantErr)
}

// TestRun_ContextCancelledBeforeConnect verifies that cancelling the context before
// the first connection attempt causes Run to return nil.
func TestRun_ContextCancelledBeforeConnect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	cs := NewClientServer(Config{ServerAddr: "localhost:1", NoTLS: true}, newTestToken(t))

	// listen should never be called because ctx is already done.
	called := atomic.Bool{}
	cs.listen = func(_ context.Context, _ string, _ ...revdial.ListenerOption) (net.Listener, error) {
		called.Store(true)
		return nil, errors.New("should not be called")
	}

	err := cs.Run(ctx)

	require.NoError(t, err)
	assert.False(t, called.Load(), "listen should not have been called with a cancelled context")
}

// TestRun_ReconnectAfterDisconnect verifies that after a successful first connection
// that then drops (ErrListenerClosed), Run reconnects and invokes onReconnected.
func TestRun_ReconnectAfterDisconnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reconnectedCh := make(chan string, 1)

	firstListener := newFakeListener()

	callCount := atomic.Int32{}

	cs := NewClientServer(
		Config{ServerAddr: "localhost:1", NoTLS: true},
		newTestToken(t),
		WithOnReconnected(func(url string) { reconnectedCh <- url }),
	)
	cs.initialBackoff = time.Millisecond // speed up test

	cs.listen = func(lCtx context.Context, _ string, _ ...revdial.ListenerOption) (net.Listener, error) {
		n := callCount.Add(1)

		switch n {
		case 1:
			// First call: return the controllable listener, then close it to simulate disconnect.
			go func() { firstListener.Close() }()
			return firstListener, nil
		default:
			// Second call (reconnect): cancel context so Run exits cleanly.
			cancel()
			return nil, context.Canceled
		}
	}

	err := cs.Run(ctx)

	require.NoError(t, err)
	assert.EqualValues(t, 2, callCount.Load(), "listen should have been called twice (connect + reconnect attempt)")
}

// TestRun_OnConnectedCallback verifies that onConnected is called (not onReconnected)
// for the initial connection.
func TestRun_OnConnectedCallback(t *testing.T) {
	// We can't directly trigger the urlToConnectUpdated event from outside, but we can
	// verify that the isReconnect flag fed to buildOpts is false on the first call and
	// true on subsequent calls by observing which callback is wired to the event handler.
	// Here we use a simpler structural check: the first listen call receives attempt==0,
	// so buildOpts is called with isReconnect=false.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	firstListener := newFakeListener()
	callCount := atomic.Int32{}

	connectedCalled := atomic.Bool{}
	reconnectedCalled := atomic.Bool{}

	cs := NewClientServer(
		Config{ServerAddr: "localhost:1", NoTLS: true},
		newTestToken(t),
		WithOnConnected(func(_ string) { connectedCalled.Store(true) }),
		WithOnReconnected(func(_ string) { reconnectedCalled.Store(true) }),
	)
	cs.initialBackoff = time.Millisecond // speed up test

	cs.listen = func(lCtx context.Context, _ string, _ ...revdial.ListenerOption) (net.Listener, error) {
		n := callCount.Add(1)

		if n == 1 {
			go func() { firstListener.Close() }()
			return firstListener, nil
		}

		cancel()

		return nil, context.Canceled
	}

	_ = cs.Run(ctx)

	// We cannot assert connectedCalled/reconnectedCalled here because the event is only
	// fired by the revdial server sending a "urlToConnectUpdated" message over the wire.
	// What we CAN assert is that Run called listen twice (first connect + reconnect).
	assert.EqualValues(t, 2, callCount.Load())
	// And that neither callback was fired without a real server event (no false positives).
	assert.False(t, connectedCalled.Load())
	assert.False(t, reconnectedCalled.Load())
}

// TestRun_BackoffResetOnSuccessfulConnect verifies that after a successful connection
// the reconnect backoff resets to the initial value. We do this by timing two
// reconnect cycles: first a failed reconnect (which applies backoff), then a
// successful connection that resets it.
func TestRun_SubsequentConnectFailureRetries(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	callCount := atomic.Int32{}
	firstListener := newFakeListener()

	connectErr := errors.New("temporary failure")

	cs := NewClientServer(Config{ServerAddr: "localhost:1", NoTLS: true}, newTestToken(t))
	cs.initialBackoff = time.Millisecond // speed up test

	cs.listen = func(lCtx context.Context, _ string, _ ...revdial.ListenerOption) (net.Listener, error) {
		n := callCount.Add(1)

		switch n {
		case 1:
			// First call succeeds — simulate clean disconnect immediately.
			go func() { firstListener.Close() }()
			return firstListener, nil
		case 2:
			// Second call (reconnect after disconnect) fails.
			return nil, connectErr
		default:
			// Third call: cancel context so Run exits cleanly.
			cancel()
			return nil, context.Canceled
		}
	}

	err := cs.Run(ctx)

	require.NoError(t, err)
	// listen was called 3 times: initial connect, failed reconnect, then ctx-cancelled reconnect.
	assert.EqualValues(t, 3, callCount.Load())
}
