package core

import (
	"context"
	"errors"
	"net"
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

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := service.HandleHTTPConnection(ctx, "test-user", clientConn, func(net.Conn) error { return nil }, "127.0.0.1")
	require.ErrorIs(t, err, ErrFailedToConnect)
}

func TestHandleHTTPConnection_WriteError(t *testing.T) {
	connManager := NewMockConnManager(t)
	authRepo := NewMockAuthRepo(t)

	revConn := conn.NewMockWithWriteCloser(t)

	mockReq := conn.NewMockRequest(t)
	connManager.EXPECT().RequestConnection(mock.Anything, "test-user").Return(mockReq, nil)
	mockReq.EXPECT().WaitConn(mock.Anything).Return(revConn, nil)

	service := New(connManager, authRepo)
	clientConn := conn.NewMockWithWriteCloser(t)

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
