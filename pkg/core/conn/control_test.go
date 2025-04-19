package conn

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ksysoev/revdial/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestServConn_ID(t *testing.T) {
	mockConn := NewMockserverConn(t)
	expectedID := uuid.New()
	mockConn.EXPECT().ID().Return(expectedID)

	sc := NewServerConn(context.Background(), mockConn)

	resultID := sc.ID()

	assert.Equal(t, expectedID, resultID)
}

func TestServConn_Context(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	mockConn := NewMockserverConn(t)

	sc := NewServerConn(ctx, mockConn)

	select {
	case <-sc.Context().Done():
	default:
		t.Error("expected context to be canceled")
	}
}

func TestServConn_Close(t *testing.T) {
	mockConn := NewMockserverConn(t)
	mockConn.EXPECT().Close().Return(nil)

	ctx := context.Background()
	sc := NewServerConn(ctx, mockConn)

	err := sc.Close()

	assert.NoError(t, err)
}

func TestServConn_RequestConnection(t *testing.T) {
	tests := []struct {
		mockSendResponse error
		expectedError    error
		name             string
		mockState        proto.State
	}{
		{name: "ServerNotRegistered", mockState: proto.StateConnected, mockSendResponse: assert.AnError, expectedError: assert.AnError},
		{name: "SendConnectCommandFails", mockState: proto.StateRegistered, mockSendResponse: assert.AnError, expectedError: assert.AnError},
		{name: "Success", mockState: proto.StateRegistered, mockSendResponse: nil, expectedError: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConn := NewMockserverConn(t)

			mockConn.EXPECT().SendConnectCommand(mock.Anything).Return(tt.mockSendResponse)

			sc := NewServerConn(context.Background(), mockConn)
			req, err := sc.RequestConnection()

			if tt.expectedError != nil {
				assert.Nil(t, req)
				assert.ErrorIs(t, err, tt.expectedError)
			} else {
				assert.NotNil(t, req)
				assert.NoError(t, err)
			}
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

func TestControlConn_Ping(t *testing.T) {
	tests := []struct {
		name          string
		mockPingError error
		expectErr     bool
	}{
		{"PingSuccess", nil, false},
		{"PingFailure", assert.AnError, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock serverConn
			mockConn := NewMockserverConn(t)

			// Arrange: Setup the mock expectation for SendPingCommand
			mockConn.EXPECT().SendPingCommand().Return(tt.mockPingError)

			// Create a ControlConn instance
			cc := NewServerConn(context.Background(), mockConn)

			// Act: Call Ping()
			err := cc.Ping()

			// Assert: Check results based on expectation
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
