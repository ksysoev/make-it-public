package connsvc

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

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
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

func TestHandleHTTPConnection_ConnectionRequestFailure(t *testing.T) {
	connManager := NewMockConnManager(t)
	authRepo := NewMockAuthRepo(t)

	connManager.EXPECT().RequestConnection(mock.Anything, "test-user").Return(nil, errors.New("connection failed"))

	service := New(connManager, authRepo)
	clientConn := &mockConn{}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := service.HandleHTTPConnection(ctx, "test-user", clientConn, func(net.Conn) error { return nil })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to request connection: connection failed")
}

func TestHandleHTTPConnection_WriteError(t *testing.T) {
	connManager := NewMockConnManager(t)
	authRepo := NewMockAuthRepo(t)

	connChan := make(chan net.Conn, 1)
	revConn := &mockConn{readData: []byte("response data")}
	connChan <- revConn
	connManager.EXPECT().RequestConnection(mock.Anything, "test-user").Return(connChan, nil)

	service := New(connManager, authRepo)
	clientConn := &mockConn{}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	writeFunc := func(conn net.Conn) error {
		return errors.New("write error")
	}

	err := service.HandleHTTPConnection(ctx, "test-user", clientConn, writeFunc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write initial request: write error")
}

func TestHandleHTTPConnection_ContextCancellation(t *testing.T) {
	connManager := NewMockConnManager(t)
	authRepo := NewMockAuthRepo(t)

	connChan := make(chan net.Conn, 1)
	connManager.EXPECT().RequestConnection(mock.Anything, "test-user").Return(connChan, nil)

	service := New(connManager, authRepo)
	clientConn := &mockConn{}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := service.HandleHTTPConnection(ctx, "test-user", clientConn, func(net.Conn) error { return nil })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}
