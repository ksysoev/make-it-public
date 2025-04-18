package connmng

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/ksysoev/make-it-public/pkg/core/conn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestConnManager_AddConnection(t *testing.T) {
	cm := New()
	mockConn := conn.NewMockServConn(t)

	mockConn.EXPECT().Close().Return(nil)

	cm.AddConnection("key1", mockConn)

	assert.NotNil(t, cm.conns["key1"])
	assert.Equal(t, mockConn, cm.conns["key1"])

	// Overwrite connection
	newConn := conn.NewMockServConn(t)

	cm.AddConnection("key1", newConn)

	assert.Equal(t, newConn, cm.conns["key1"])
}

func TestConnManager_RemoveConnection(t *testing.T) {
	cm := New()
	mockConn := conn.NewMockServConn(t)

	connID := uuid.New()
	mockConn.EXPECT().ID().Return(connID)
	mockConn.EXPECT().Close().Return(nil)

	cm.AddConnection("key1", mockConn)
	cm.RemoveConnection("key1", connID)

	assert.Nil(t, cm.conns["key1"])
}

func TestConnManager_RequestConnection(t *testing.T) {
	mockConn := conn.NewMockServConn(t)
	mockReq := conn.NewMockConnReq(t)
	cm := New()

	reqID := uuid.New()

	mockConn.EXPECT().RequestConnection().Return(mockReq, nil)
	mockReq.EXPECT().ID().Return(reqID)

	cm.AddConnection("key1", mockConn)

	req, err := cm.RequestConnection(context.Background(), "key1")

	require.NoError(t, err)
	assert.Equal(t, mockReq, req)
	assert.NotNil(t, cm.requests[reqID])
}

func TestConnManager_RequestConnection_NoConnection(t *testing.T) {
	cm := New()
	_, err := cm.RequestConnection(context.Background(), "key1")

	assert.ErrorContains(t, err, "no connections for user")
}

func TestConnManager_RequestConnection_Error(t *testing.T) {
	mockConn := conn.NewMockServConn(t)
	cm := New()

	mockConn.EXPECT().RequestConnection().Return(nil, errors.New("connection error"))
	cm.AddConnection("key1", mockConn)

	_, err := cm.RequestConnection(context.Background(), "key1")

	assert.ErrorContains(t, err, "failed to send connect command")
}

func TestConnManager_ResolveRequest(t *testing.T) {
	mockReq := conn.NewMockConnReq(t)
	cm := New()

	reqID := uuid.New()
	cm.requests[reqID] = &connRequest{
		ctx: context.Background(),
		req: mockReq,
	}

	mockReq.EXPECT().SendConn(mock.Anything, mock.Anything).Return()

	revConn := new(net.TCPConn)
	cm.ResolveRequest(reqID, revConn)

	assert.Nil(t, cm.requests[reqID])
}

func TestConnManager_CancelRequest(t *testing.T) {
	mockReq := conn.NewMockConnReq(t)
	cm := New()

	reqID := uuid.New()
	cm.requests[reqID] = &connRequest{
		ctx: context.Background(),
		req: mockReq,
	}

	mockReq.EXPECT().Cancel()

	cm.CancelRequest(reqID)

	assert.Nil(t, cm.requests[reqID])
}

func TestConnManager_Close(t *testing.T) {
	mockConn := conn.NewMockServConn(t)
	mockReq := conn.NewMockConnReq(t)
	cm := New()

	reqID := uuid.New()

	mockConn.EXPECT().Close().Return(nil)
	mockReq.EXPECT().Cancel().Return()

	cm.conns["key1"] = mockConn
	cm.requests[reqID] = &connRequest{
		ctx: context.Background(),
		req: mockReq,
	}

	err := cm.Close()

	require.NoError(t, err)
	assert.Empty(t, cm.requests)
}
