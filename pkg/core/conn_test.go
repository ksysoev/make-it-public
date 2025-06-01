package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ksysoev/make-it-public/pkg/core/conn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockConn implements net.Conn interface for testing
type mockConn struct {
	mock.Mock
	readData  []byte
	writeData []byte
	closed    bool
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	if len(m.readData) == 0 {
		return 0, io.EOF
	}

	n = copy(b, m.readData)
	m.readData = m.readData[n:]

	return n, nil
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	m.writeData = append(m.writeData, b...)
	return len(b), nil
}

func (m *mockConn) Close() error {
	m.closed = true
	return nil
}

func (m *mockConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (m *mockConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }
func (m *mockConn) SetDeadline(_ time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(_ time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(_ time.Time) error { return nil }

func TestHandleHTTPConnection_ConnectionRequestFailure(t *testing.T) {
	connManager := NewMockConnManager(t)
	authRepo := NewMockAuthRepo(t)

	connManager.EXPECT().RequestConnection(mock.Anything, "test-user").Return(nil, errors.New("connection failed"))

	service := New(connManager, authRepo)
	clientConn := &mockConn{}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := service.HandleHTTPConnection(ctx, "test-user", clientConn, func(net.Conn) error { return nil }, "127.0.0.1")
	require.ErrorIs(t, err, ErrFailedToConnect)
}

func TestHandleHTTPConnection_WriteError(t *testing.T) {
	connManager := NewMockConnManager(t)
	authRepo := NewMockAuthRepo(t)

	revConn := &mockConn{readData: []byte("response data")}

	mockReq := conn.NewMockRequest(t)
	connManager.EXPECT().RequestConnection(mock.Anything, "test-user").Return(mockReq, nil)
	mockReq.EXPECT().WaitConn(mock.Anything).Return(revConn, nil)

	service := New(connManager, authRepo)
	clientConn := &mockConn{}

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
	clientConn := &mockConn{}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := service.HandleHTTPConnection(ctx, "test-user", clientConn, func(net.Conn) error { return nil }, "127.0.0.1")
	require.ErrorIs(t, err, ErrFailedToConnect)
}

func TestPipeConn(t *testing.T) {
	tests := []struct {
		src         io.Reader
		dst         io.Writer
		expectErr   error
		name        string
		expectBytes int64
	}{
		{
			name:        "successfully copies data",
			src:         bytes.NewReader([]byte("sample data")),
			dst:         &bytes.Buffer{},
			expectErr:   io.EOF,
			expectBytes: int64(len("sample data")),
		},
		{
			name:        "error on closed pipe",
			src:         bytes.NewReader([]byte("")),
			dst:         nil,
			expectErr:   fmt.Errorf("error copying from reverse connection: %w", syscall.ECONNRESET),
			expectBytes: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copyFunc := pipeConn(tt.src, tt.dst)

			err := copyFunc()
			if tt.expectErr != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTimeoutContext(t *testing.T) {
	tests := []struct {
		name           string
		timeout        time.Duration
		cancelEarly    bool
		cancelOriginal bool
		sleepBefore    time.Duration
		expectCanceled bool
	}{
		{
			name:           "zero timeout",
			timeout:        0,
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

			ctx, cancel := timeoutContext(origCtx, tt.timeout)
			if tt.cancelEarly {
				cancel()
			} else {
				defer cancel()
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
