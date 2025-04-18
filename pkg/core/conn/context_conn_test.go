package conn

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockConn struct {
	net.Conn
	mock.Mock
}

func (m *MockConn) Read(b []byte) (int, error) {
	args := m.Called(b)
	return args.Int(0), args.Error(1)
}

func (m *MockConn) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockConn) SetReadDeadline(t time.Time) error {
	args := m.Called(t)
	return args.Error(0)
}

func TestRespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockConn := new(MockConn)
	mockConn.On("SetReadDeadline", mock.Anything).Return(nil)
	mockConn.On("Read", mock.Anything).Return(0, errors.New("read error"))

	conn := NewContextConnNopCloser(ctx, mockConn)

	cancel()

	_, err := conn.Read(make([]byte, 10))
	assert.ErrorIs(t, err, context.Canceled)
}

func TestRespectsContextDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	mockConn := new(MockConn)
	mockConn.On("SetReadDeadline", mock.Anything).Return(nil)
	mockConn.On("Read", mock.Anything).Return(0, os.ErrDeadlineExceeded)

	conn := NewContextConnNopCloser(ctx, mockConn)

	time.Sleep(60 * time.Millisecond)

	_, err := conn.Read(make([]byte, 10))

	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestReturnsDataWhenReadSucceeds(t *testing.T) {
	ctx := context.Background()

	mockConn := new(MockConn)
	mockConn.On("SetReadDeadline", mock.Anything).Return(nil)
	mockConn.On("Read", mock.Anything).Return(5, nil)

	conn := NewContextConnNopCloser(ctx, mockConn)

	data := make([]byte, 10)
	n, err := conn.Read(data)
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
}

func TestCloseCancelsContext(t *testing.T) {
	ctx := context.Background()

	mockConn := new(MockConn)
	mockConn.On("Close").Return(nil)

	conn := NewContextConnNopCloser(ctx, mockConn)

	_ = conn.Close()
	assert.ErrorIs(t, conn.ctx.Err(), context.Canceled)
}

func TestHandlesSetReadDeadlineError(t *testing.T) {
	ctx := context.Background()

	mockConn := new(MockConn)
	expectedErr := errors.New("deadline setting error")
	mockConn.On("SetReadDeadline", mock.Anything).Return(expectedErr)

	conn := NewContextConnNopCloser(ctx, mockConn)

	_, err := conn.Read(make([]byte, 10))
	assert.ErrorIs(t, err, expectedErr)
}

func TestContinuesReadingAfterDeadlineExceeded(t *testing.T) {
	ctx := context.Background()

	mockConn := new(MockConn)
	mockConn.On("SetReadDeadline", mock.Anything).Return(nil)
	mockConn.On("Read", mock.Anything).Return(10, os.ErrDeadlineExceeded).Once()

	conn := NewContextConnNopCloser(ctx, mockConn)

	data := make([]byte, 10)
	n, err := conn.Read(data)
	assert.Equal(t, 10, n)
	assert.NoError(t, err)
}

func TestUsesContextDeadlineWhenAvailable(t *testing.T) {
	deadline := time.Now().Add(5 * time.Millisecond)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)

	defer cancel()

	mockConn := new(MockConn)
	mockConn.On("SetReadDeadline", deadline).Return(nil)
	mockConn.On("Read", mock.Anything).Return(3, nil)

	conn := NewContextConnNopCloser(ctx, mockConn)

	n, err := conn.Read(make([]byte, 10))
	assert.NoError(t, err)
	assert.Equal(t, 3, n)
	mockConn.AssertCalled(t, "SetReadDeadline", deadline)
}

func TestUsesDefaultDeadlineWhenNoContextDeadline(t *testing.T) {
	ctx := context.Background()

	mockConn := new(MockConn)
	mockConn.On("SetReadDeadline", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		deadline, ok := args.Get(0).(time.Time)
		require.True(t, ok)

		expectedMinDeadline := time.Now().Add(interval - time.Millisecond)
		assert.True(t, deadline.After(expectedMinDeadline))
	})
	mockConn.On("Read", mock.Anything).Return(3, nil)

	conn := NewContextConnNopCloser(ctx, mockConn)

	_, err := conn.Read(make([]byte, 10))
	assert.NoError(t, err)
}

func TestReturnsTrueErrorFromRead(t *testing.T) {
	ctx := context.Background()

	mockConn := new(MockConn)
	mockConn.On("SetReadDeadline", mock.Anything).Return(nil)

	expectedErr := errors.New("connection reset")
	mockConn.On("Read", mock.Anything).Return(0, expectedErr)

	conn := NewContextConnNopCloser(ctx, mockConn)

	_, err := conn.Read(make([]byte, 10))
	assert.ErrorIs(t, err, expectedErr)
}
