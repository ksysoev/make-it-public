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
	pipeFunc := pipeToDest(src, dst)
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
	pipeFunc := pipeToDest(src, dst)
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
	pipeFunc := pipeToDest(src, dst)
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
	pipeFunc := pipeToDest(src, dst)
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
	pipeFunc := pipeToDest(src, dst)
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
	pipeFunc := pipeToDest(src, dst)
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
	pipeFunc := pipeToSource(src, &buf, &written)
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
	pipeFunc := pipeToSource(src, &buf, &written)
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
	pipeFunc := pipeToSource(src, &buf, &written)
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
	pipeFunc := pipeToSource(src, &buf, &written)
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
