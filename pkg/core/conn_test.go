package core

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ksysoev/make-it-public/pkg/core/conn"
	"github.com/ksysoev/revdial/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHandleHTTPConnection_ConnectionRequestFailure(t *testing.T) {
	connManager := NewMockConnManager(t)
	authRepo := NewMockAuthRepo(t)

	connManager.EXPECT().RequestConnection(mock.Anything, "test-user").Return(nil, errors.New("connection failed"))

	service := New(connManager, authRepo)
	clientConn := conn.NewMockWithWriteCloser(t)

	clientConn.EXPECT().RemoteAddr().Return(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := service.HandleHTTPConnection(ctx, "test-user", clientConn, func(net.Conn) error { return nil }, "127.0.0.1")
	require.ErrorIs(t, err, ErrFailedToConnect)
}

func TestHandleHTTPConnection_WriteError(t *testing.T) {
	connManager := NewMockConnManager(t)
	authRepo := NewMockAuthRepo(t)

	revConn := conn.NewMockWithWriteCloser(t)
	revConn.EXPECT().Write(mock.Anything).Return(0, assert.AnError)

	mockReq := conn.NewMockRequest(t)
	connManager.EXPECT().RequestConnection(mock.Anything, "test-user").Return(mockReq, nil)
	mockReq.EXPECT().WaitConn(mock.Anything).Return(revConn, nil)

	service := New(connManager, authRepo)
	clientConn := conn.NewMockWithWriteCloser(t)

	clientConn.EXPECT().RemoteAddr().Return(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	writeFunc := func(_ net.Conn) error {
		return assert.AnError
	}

	err := service.HandleHTTPConnection(ctx, "test-user", clientConn, writeFunc, "127.0.0.1")
	assert.ErrorIs(t, err, ErrFailedToConnect)
}

func TestHandleHTTPConnection_ContextCancellation(t *testing.T) {
	connManager := NewMockConnManager(t)
	authRepo := NewMockAuthRepo(t)

	reqID := uuid.New()
	mockReq := conn.NewMockRequest(t)
	mockReq.EXPECT().ID().Return(reqID)
	mockReq.EXPECT().WaitConn(mock.Anything).Return(nil, context.Canceled)

	connManager.EXPECT().RequestConnection(mock.Anything, "test-user").Return(mockReq, nil)
	connManager.EXPECT().CancelRequest(reqID).Return()

	service := New(connManager, authRepo)
	clientConn := conn.NewMockWithWriteCloser(t)

	clientConn.EXPECT().RemoteAddr().Return(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := service.HandleHTTPConnection(ctx, "test-user", clientConn, func(net.Conn) error { return nil }, "127.0.0.1")
	require.ErrorIs(t, err, ErrFailedToConnect)
}

func TestTimeoutContext(t *testing.T) {
	tests := []struct {
		name           string
		timeout        time.Duration
		sleepBefore    time.Duration
		cancelEarly    bool
		cancelOriginal bool
		expectCanceled bool
	}{
		{
			name:           "zero timeout",
			timeout:        0,
			sleepBefore:    10 * time.Millisecond,
			expectCanceled: false,
		},
		{
			name:           "negative timeout",
			timeout:        -10 * time.Millisecond,
			sleepBefore:    10 * time.Millisecond,
			expectCanceled: false,
		},
		{
			name:           "valid timeout",
			timeout:        50 * time.Millisecond,
			sleepBefore:    60 * time.Millisecond,
			expectCanceled: true,
		},
		{
			name:           "early cancel before timeout",
			timeout:        100 * time.Millisecond,
			cancelEarly:    true,
			sleepBefore:    50 * time.Millisecond,
			expectCanceled: false,
		},
		{
			name:           "original context canceled",
			timeout:        100 * time.Millisecond,
			cancelOriginal: true,
			sleepBefore:    50 * time.Millisecond,
			expectCanceled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origCtx, origCancel := context.WithCancel(context.Background())
			defer origCancel()

			ctx, cancelTimeout := timeoutContext(origCtx, tt.timeout)
			if tt.cancelEarly {
				cancelTimeout()
			} else {
				defer cancelTimeout()
			}

			if tt.cancelOriginal {
				origCancel()
			}

			time.Sleep(tt.sleepBefore)

			select {
			case <-ctx.Done():
				if !tt.expectCanceled {
					t.Errorf("unexpected cancellation: %v", ctx.Err())
				}
			default:
				if tt.expectCanceled {
					t.Errorf("expected cancellation did not occur")
				}
			}
		})
	}
}

func TestTimeoutContext_ResourceCleanup(t *testing.T) {
	ctx := context.Background()
	timeout := 1 * time.Hour

	initialGoroutines := runtime.NumGoroutine()
	_, cancel := timeoutContext(ctx, timeout)

	time.Sleep(10 * time.Millisecond)

	goroutinesAfterCreate := runtime.NumGoroutine()

	assert.Greater(t, goroutinesAfterCreate, initialGoroutines, "Expected a new goroutine to be created")

	cancel()

	time.Sleep(10 * time.Millisecond)

	goroutinesAfterCancel := runtime.NumGoroutine()

	assert.Equal(t, initialGoroutines, goroutinesAfterCancel, "Expected goroutine to be cleaned up after cancellation")
}

func TestTimeoutContext_ErrorPropagation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately to set the error

	timeoutCtx, timeoutCancel := timeoutContext(ctx, 100*time.Millisecond)
	defer timeoutCancel()

	assert.Equal(t, ctx.Err(), timeoutCtx.Err(), "Expected error from parent context to be propagated")
}

func TestTimeoutContext_ImmediateCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	timeoutCtx, timeoutCancel := timeoutContext(ctx, 100*time.Millisecond)
	defer timeoutCancel()

	// Verify that the timeout context is immediately canceled
	select {
	case <-timeoutCtx.Done():
		// Expected behavior
	default:
		t.Error("Expected timeout context to be immediately canceled when parent is already canceled")
	}
}

func TestTimeoutContext_ConcurrentUsage(t *testing.T) {
	const numGoroutines = 10

	wg := &sync.WaitGroup{}
	wg.Add(numGoroutines)

	ctx := context.Background()

	// Launch multiple goroutines that create and use timeout contexts
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			// Create a timeout context
			timeoutCtx, cancel := timeoutContext(ctx, time.Duration(id+1)*10*time.Millisecond)
			defer cancel()

			// Wait for either the timeout or a fixed duration
			select {
			case <-timeoutCtx.Done():
				// Context was canceled by timeout, which is expected for shorter timeouts
			case <-time.After(100 * time.Millisecond):
				// This should only happen for the longer timeouts
				if id < 5 { // First 5 should timeout before 100ms
					t.Errorf("Expected timeout for goroutine %d", id)
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
}

func TestPipeToDest_SuccessfulCopy(t *testing.T) {
	// Create a mock destination
	dst := conn.NewMockWithWriteCloser(t)

	// Create a source with test data
	testData := []byte("test data")
	src := bytes.NewReader(testData)

	// Set up expectations
	dst.EXPECT().Write(mock.Anything).Run(func(b []byte) {
		assert.Equal(t, testData, b)
	}).Return(len(testData), nil)
	dst.EXPECT().CloseWrite().Return(nil)

	// Execute the function
	pipeFunc := pipeToDest(t.Context(), src, dst)
	err := pipeFunc()

	// Verify results
	assert.NoError(t, err)
}

func TestPipeToDest_NetErrClosed(t *testing.T) {
	// Create a mock destination
	dst := conn.NewMockWithWriteCloser(t)

	// Create a source with some data
	src := bytes.NewReader([]byte("test data"))

	// Set up expectations
	dst.EXPECT().Write(mock.Anything).Return(0, net.ErrClosed)

	// Execute the function
	pipeFunc := pipeToDest(t.Context(), src, dst)
	err := pipeFunc()

	// Verify results
	assert.ErrorIs(t, err, ErrConnClosed)
}

func TestPipeToDest_EconnReset(t *testing.T) {
	// Create a mock destination
	dst := conn.NewMockWithWriteCloser(t)

	// Create a source with some data
	src := bytes.NewReader([]byte("test data"))

	// Set up expectations
	dst.EXPECT().Write(mock.Anything).Return(0, syscall.ECONNRESET)

	// Execute the function
	pipeFunc := pipeToDest(t.Context(), src, dst)
	err := pipeFunc()

	// Verify results
	assert.ErrorIs(t, err, ErrConnClosed)
}

func TestPipeToDest_OtherCopyError(t *testing.T) {
	// Create a mock destination
	dst := conn.NewMockWithWriteCloser(t)

	// Create a source with some data
	src := bytes.NewReader([]byte("test data"))

	// Set up expectations
	customErr := errors.New("custom error")
	dst.EXPECT().Write(mock.Anything).Return(0, customErr)

	// Execute the function
	pipeFunc := pipeToDest(t.Context(), src, dst)
	err := pipeFunc()

	// Verify results
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error copying from reverse connection")
	assert.ErrorIs(t, err, customErr)
}

func TestPipeToDest_CloseWriteError(t *testing.T) {
	// Create a mock destination
	dst := conn.NewMockWithWriteCloser(t)

	// Create a source with test data
	testData := []byte("test data")
	src := bytes.NewReader(testData)

	// Set up expectations
	dst.EXPECT().Write(mock.Anything).Return(len(testData), nil)

	closeErr := errors.New("close write error")
	dst.EXPECT().CloseWrite().Return(closeErr)

	// Execute the function
	pipeFunc := pipeToDest(t.Context(), src, dst)
	err := pipeFunc()

	// Verify results
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to close write end of reverse connection")
	assert.ErrorIs(t, err, closeErr)
}

func TestPipeToDest_CloseWriteNetErrClosed(t *testing.T) {
	// Create a mock destination
	dst := conn.NewMockWithWriteCloser(t)

	// Create a source with test data
	testData := []byte("test data")
	src := bytes.NewReader(testData)

	// Set up expectations
	dst.EXPECT().Write(mock.Anything).Return(len(testData), nil)
	dst.EXPECT().CloseWrite().Return(net.ErrClosed)

	// Execute the function
	pipeFunc := pipeToDest(t.Context(), src, dst)
	err := pipeFunc()

	// Verify results
	assert.NoError(t, err)
}

func TestPipeToSource_SuccessfulCopy(t *testing.T) {
	// Create a mock source
	src := conn.NewMockWithWriteCloser(t)

	// Create a buffer to capture written data
	var buf bytes.Buffer

	// Test data to be read from source
	testData := []byte("test data")

	// Set up expectations
	src.EXPECT().Read(mock.Anything).Run(func(b []byte) {
		copy(b, testData)
	}).Return(len(testData), nil).Once()

	// Return EOF on second read to end the copy operation
	src.EXPECT().Read(mock.Anything).Return(0, io.EOF).Once()

	// Track bytes written
	var written int64

	// Execute the function
	pipeFunc := pipeToSource(t.Context(), src, &buf, &written)
	err := pipeFunc()

	// Verify results
	assert.Equal(t, ErrConnClosed, err)
	assert.Equal(t, int64(len(testData)), written)
	assert.Equal(t, testData, buf.Bytes())
}

func TestPipeToSource_NetErrClosed(t *testing.T) {
	// Create a mock source
	src := conn.NewMockWithWriteCloser(t)

	// Create a destination buffer
	var buf bytes.Buffer

	// Set up expectations
	src.EXPECT().Read(mock.Anything).Return(0, net.ErrClosed)

	// Track bytes written
	var written int64

	// Execute the function
	pipeFunc := pipeToSource(t.Context(), src, &buf, &written)
	err := pipeFunc()

	// Verify results
	assert.ErrorIs(t, err, ErrConnClosed)
	assert.Equal(t, int64(0), written)
}

func TestPipeToSource_EconnReset(t *testing.T) {
	// Create a mock source
	src := conn.NewMockWithWriteCloser(t)

	// Create a destination buffer
	var buf bytes.Buffer

	// Set up expectations
	src.EXPECT().Read(mock.Anything).Return(0, syscall.ECONNRESET)

	// Track bytes written
	var written int64

	// Execute the function
	pipeFunc := pipeToSource(t.Context(), src, &buf, &written)
	err := pipeFunc()

	// Verify results
	assert.ErrorIs(t, err, ErrConnClosed)
	assert.Equal(t, int64(0), written)
}

func TestPipeToSource_OtherError(t *testing.T) {
	// Create a mock source
	src := conn.NewMockWithWriteCloser(t)

	// Create a destination buffer
	var buf bytes.Buffer

	// Set up expectations
	customErr := errors.New("custom read error")
	src.EXPECT().Read(mock.Anything).Return(0, customErr)

	// Track bytes written
	var written int64

	// Execute the function
	pipeFunc := pipeToSource(t.Context(), src, &buf, &written)
	err := pipeFunc()

	// Verify results
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error copying to reverse connection")
	assert.ErrorIs(t, err, customErr)
	assert.Equal(t, int64(0), written)
}

func TestCloseOnContextDone_ReqCtxCancellation(t *testing.T) {
	reqCtx, reqCancel := context.WithCancel(context.Background())
	parentCtx := context.Background()
	mockConn := conn.NewMockWithWriteCloser(t)

	mockConn.EXPECT().Close().Return(nil)

	wg := closeOnContextDone(reqCtx, parentCtx, mockConn)

	reqCancel()
	wg.Wait()
}

func TestCloseOnContextDone_ParentCtxCancellation(t *testing.T) {
	reqCtx := context.Background()
	parentCtx, parentCancel := context.WithCancel(context.Background())
	mockConn := conn.NewMockWithWriteCloser(t)

	mockConn.EXPECT().Close().Return(nil)

	wg := closeOnContextDone(reqCtx, parentCtx, mockConn)

	parentCancel()

	wg.Wait()
}

func TestCloseOnContextDone_CloseError(t *testing.T) {
	reqCtx, reqCancel := context.WithCancel(context.Background())
	parentCtx := context.Background()

	mockConn := conn.NewMockWithWriteCloser(t)

	closeErr := errors.New("close error")
	mockConn.EXPECT().Close().Return(closeErr)

	wg := closeOnContextDone(reqCtx, parentCtx, mockConn)

	reqCancel()

	wg.Wait()
}

// --- V2 tests ---

func TestYamuxStreamWrapper_CloseWrite(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()

	wrapper := &yamuxStreamWrapper{Conn: server}

	err := wrapper.CloseWrite()
	assert.NoError(t, err, "CloseWrite should close the write side of the connection")

	// Verify that further writes fail after CloseWrite (write side is closed)
	_, err = wrapper.Write([]byte("test"))
	assert.Error(t, err, "write should fail after CloseWrite when the write side is closed")
}

func TestYamuxStreamWrapper_DelegatesRead(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	wrapper := &yamuxStreamWrapper{Conn: server}

	go func() {
		_, _ = client.Write([]byte("hello"))
	}()

	buf := make([]byte, 5)
	n, err := wrapper.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, []byte("hello"), buf)
}

func TestYamuxStreamWrapper_DelegatesWrite(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	wrapper := &yamuxStreamWrapper{Conn: server}

	go func() {
		buf := make([]byte, 5)
		_, _ = client.Read(buf)
	}()

	n, err := wrapper.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)
}

func TestYamuxStreamWrapper_DelegatesClose(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()

	wrapper := &yamuxStreamWrapper{Conn: server}

	err := wrapper.Close()
	assert.NoError(t, err)

	// Verify the underlying connection is actually closed
	_, err = server.Read(make([]byte, 1))
	assert.Error(t, err)
}

func TestYamuxStreamWrapper_ImplementsWithWriteCloser(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	wrapper := &yamuxStreamWrapper{Conn: server}

	// Verify it satisfies the WithWriteCloser interface
	var _ conn.WithWriteCloser = wrapper
}

// buildBindMessage constructs the wire-format bind message that a V2 client sends on a yamux stream.
// Format: [VersionV1 (1 byte)][CmdBind (1 byte)][UUID (16 bytes)]
func buildBindMessage(id uuid.UUID) []byte {
	msg := make([]byte, 0, 18)
	msg = append(msg, proto.VersionV1(), proto.CmdBind())
	msg = append(msg, id[:]...)

	return msg
}

func TestHandleV2Stream_Success(t *testing.T) {
	connManager := NewMockConnManager(t)
	authRepo := NewMockAuthRepo(t)
	service := New(connManager, authRepo)

	streamServer, streamClient := net.Pipe()
	defer streamClient.Close()

	reqID := uuid.New()
	bindMsg := buildBindMessage(reqID)

	// ResolveRequest should be called with the correct UUID and a CloseNotifier wrapping the stream
	connManager.EXPECT().ResolveRequest(reqID, mock.AnythingOfType("*conn.CloseNotifier")).Run(func(_ uuid.UUID, c conn.WithWriteCloser) {
		// Close the notifier to unblock WaitClose
		_ = c.Close()
	}).Return()

	// Write bind message from client side, then read response
	go func() {
		_, err := streamClient.Write(bindMsg)
		assert.NoError(t, err)

		// Read success response: [VersionV1][ResSuccess]
		resp := make([]byte, 2)
		_, err = io.ReadFull(streamClient, resp)
		assert.NoError(t, err)
		assert.Equal(t, proto.VersionV1(), resp[0])
		assert.Equal(t, proto.ResSuccess(), resp[1])
	}()

	ctx := t.Context()
	service.handleV2Stream(ctx, streamServer, "test-key")
}

func TestHandleV2Stream_InvalidVersion(t *testing.T) {
	connManager := NewMockConnManager(t)
	authRepo := NewMockAuthRepo(t)
	service := New(connManager, authRepo)

	streamServer, streamClient := net.Pipe()
	defer streamClient.Close()

	// Send invalid version byte (0xFF instead of VersionV1)
	go func() {
		_, _ = streamClient.Write([]byte{0xFF, proto.CmdBind()})
	}()

	ctx := t.Context()
	service.handleV2Stream(ctx, streamServer, "test-key")

	// Stream should be closed by handleV2Stream due to invalid version
	_, err := streamServer.Read(make([]byte, 1))
	assert.Error(t, err, "stream should be closed after invalid version")
}

func TestHandleV2Stream_InvalidCommand(t *testing.T) {
	connManager := NewMockConnManager(t)
	authRepo := NewMockAuthRepo(t)
	service := New(connManager, authRepo)

	streamServer, streamClient := net.Pipe()
	defer streamClient.Close()

	// Send valid version but wrong command (0xFF instead of CmdBind)
	go func() {
		_, _ = streamClient.Write([]byte{proto.VersionV1(), 0xFF})
	}()

	ctx := t.Context()
	service.handleV2Stream(ctx, streamServer, "test-key")

	// Stream should be closed
	_, err := streamServer.Read(make([]byte, 1))
	assert.Error(t, err, "stream should be closed after invalid command")
}

func TestHandleV2Stream_TruncatedUUID(t *testing.T) {
	connManager := NewMockConnManager(t)
	authRepo := NewMockAuthRepo(t)
	service := New(connManager, authRepo)

	streamServer, streamClient := net.Pipe()

	// Send version + command + only 5 bytes of UUID, then close
	go func() {
		_, _ = streamClient.Write([]byte{proto.VersionV1(), proto.CmdBind()})
		_, _ = streamClient.Write([]byte{1, 2, 3, 4, 5})
		_ = streamClient.Close()
	}()

	ctx := t.Context()
	service.handleV2Stream(ctx, streamServer, "test-key")

	// Stream should be closed because UUID read failed
	_, err := streamServer.Read(make([]byte, 1))
	assert.Error(t, err, "stream should be closed after truncated UUID")
}

func TestHandleV2Stream_WriteResponseFails(t *testing.T) {
	connManager := NewMockConnManager(t)
	authRepo := NewMockAuthRepo(t)
	service := New(connManager, authRepo)

	streamServer, streamClient := net.Pipe()

	reqID := uuid.New()
	bindMsg := buildBindMessage(reqID)

	// Write bind message then immediately close so the response write fails
	go func() {
		_, _ = streamClient.Write(bindMsg)
		_ = streamClient.Close()
	}()

	ctx := t.Context()
	service.handleV2Stream(ctx, streamServer, "test-key")

	// Stream should be closed regardless
	_, err := streamServer.Read(make([]byte, 1))
	assert.Error(t, err, "stream should be closed after write failure")
}

func TestHandleV2Stream_EmptyStream(t *testing.T) {
	connManager := NewMockConnManager(t)
	authRepo := NewMockAuthRepo(t)
	service := New(connManager, authRepo)

	streamServer, streamClient := net.Pipe()

	// Close immediately with no data
	go func() {
		_ = streamClient.Close()
	}()

	ctx := t.Context()
	service.handleV2Stream(ctx, streamServer, "test-key")

	// Should return without panic and stream should be closed
	_, err := streamServer.Read(make([]byte, 1))
	assert.Error(t, err, "stream should be closed after empty stream")
}

func TestAcceptV2Streams_ContextCancellation(t *testing.T) {
	// Test that acceptV2Streams respects context cancellation.
	// We use a mock ServerV2 to avoid race conditions in the revdial library.
	connManager := NewMockConnManager(t)
	authRepo := NewMockAuthRepo(t)
	service := New(connManager, authRepo)

	// Create a real ServerV2 that will block on AcceptStream
	serverPipe, clientPipe := net.Pipe()
	defer serverPipe.Close()
	defer clientPipe.Close()

	baseOpts := []proto.ServerOption{
		proto.WithUserPassAuth(func(_, _ string) bool {
			return true
		}),
	}
	servConn := proto.NewServerV2(serverPipe, baseOpts)

	// We don't actually need to complete V2 negotiation - just test that context cancellation works
	// Create a cancelled context to test the immediate return path
	acceptCtx, acceptCancel := context.WithCancel(t.Context())
	acceptCancel() // Cancel immediately

	done := make(chan struct{})

	go func() {
		service.acceptV2Streams(acceptCtx, servConn, "testkey")
		close(done)
	}()

	// Verify it returns immediately due to cancelled context
	select {
	case <-done:
		// acceptV2Streams exited as expected
	case <-time.After(1 * time.Second):
		t.Fatal("acceptV2Streams did not return after context cancellation")
	}
}
