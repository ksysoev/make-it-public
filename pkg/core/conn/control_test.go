package conn

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ksysoev/revdial/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockServerConn struct {
	mock.Mock
}

func (m *mockServerConn) ID() uuid.UUID {
	args := m.Called()
	id, ok := args.Get(0).(uuid.UUID)

	if !ok {
		panic("expected uuid.UUID")
	}

	return id
}

func (m *mockServerConn) Close() error {
	args := m.Called()

	return args.Error(0)
}

func (m *mockServerConn) SendConnectCommand(id uuid.UUID) error {
	args := m.Called(id)

	return args.Error(0)
}

func (m *mockServerConn) State() proto.State {
	args := m.Called()
	s, ok := args.Get(0).(proto.State)

	if !ok {
		panic("expected proto.State")
	}

	return s
}

func TestServConn_ID(t *testing.T) {
	mockConn := new(mockServerConn)
	expectedID := uuid.New()
	mockConn.On("ID").Return(expectedID)

	sc := NewServerConn(context.Background(), mockConn)

	resultID := sc.ID()

	assert.Equal(t, expectedID, resultID)
	mockConn.AssertExpectations(t)
}

func TestServConn_Context(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	mockConn := new(mockServerConn)

	sc := NewServerConn(ctx, mockConn)

	select {
	case <-sc.Context().Done():
	default:
		t.Error("expected context to be canceled")
	}
}

func TestServConn_Close(t *testing.T) {
	mockConn := new(mockServerConn)
	mockConn.On("Close").Return(nil)

	ctx := context.Background()
	sc := NewServerConn(ctx, mockConn)

	err := sc.Close()

	assert.NoError(t, err)
	mockConn.AssertExpectations(t)
}

func TestServConn_RequestConnection(t *testing.T) {
	tests := []struct {
		mockSendResponse error
		expectedError    error
		name             string
		mockState        proto.State
	}{
		{name: "ServerNotRegistered", mockState: proto.StateConnected, mockSendResponse: nil, expectedError: errors.New("server is not connected")},
		{name: "SendConnectCommandFails", mockState: proto.StateRegistered, mockSendResponse: errors.New("send error"), expectedError: errors.New("failed to send connect command: send error")},
		{name: "Success", mockState: proto.StateRegistered, mockSendResponse: nil, expectedError: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := new(mockServerConn)
			mockConn.On("State").Return(tt.mockState)

			if tt.mockState == proto.StateRegistered {
				mockConn.On("SendConnectCommand", mock.Anything).Return(tt.mockSendResponse)
			}

			sc := NewServerConn(context.Background(), mockConn)
			req, err := sc.RequestConnection()

			if tt.expectedError != nil {
				assert.Nil(t, req)
				assert.EqualError(t, err, tt.expectedError.Error())
			} else {
				assert.NotNil(t, req)
				assert.NoError(t, err)
			}

			mockConn.AssertExpectations(t)
		})
	}
}

func TestCloseNotifier_WaitClose(t *testing.T) {
	tests := []struct {
		name       string
		ctxTimeout time.Duration
		closeDelay time.Duration
	}{
		{"CloseBeforeContextTimeout", 2 * time.Second, 1 * time.Second},
		{"ContextTimeoutBeforeClose", 1 * time.Second, 2 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := &net.IPConn{}
			ctx, cancel := context.WithTimeout(context.Background(), tt.ctxTimeout)

			defer cancel()

			cn := NewCloseNotifier(mockConn)

			go func() {
				time.Sleep(tt.closeDelay)

				_ = cn.Close()
			}()

			cn.WaitClose(ctx)

			select {
			case <-ctx.Done():
				assert.Error(t, ctx.Err())
			case <-cn.done:
				assert.True(t, true)
			}
		})
	}
}

func TestCloseNotifier_Close(t *testing.T) {
	mockConn := &net.IPConn{}

	cn := NewCloseNotifier(mockConn)
	_ = cn.Close()

	select {
	case <-cn.done:
	default:
		t.Fail()
	}
}
