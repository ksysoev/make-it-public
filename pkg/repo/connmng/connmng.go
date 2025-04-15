package connmng

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/ksysoev/make-it-public/pkg/core"
)

type connRequest struct {
	ctx context.Context
	ch  chan *core.ClientConn
}

type ConnManager struct {
	conns    map[string]*core.ServConn
	requests map[uuid.UUID]*connRequest
	mu       sync.RWMutex
}

// New creates and returns a new instance of ConnManager.
// It does not take any parameters.
// It returns a pointer to a ConnManager with initialized internal maps for conns and requests.
func New() *ConnManager {
	return &ConnManager{
		conns:    make(map[string]*core.ServConn),
		requests: make(map[uuid.UUID]*connRequest),
	}
}

// AddConnection adds a server connection to the user's connection pool.
// It takes a user parameter of type string and a conn parameter of type *proto.Server.
// It does not return any value and ensures thread-safe access.
func (cm *ConnManager) AddConnection(keyID string, conn *core.ServConn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if oldConn, ok := cm.conns[keyID]; ok {
		_ = oldConn.Close()
	}

	cm.conns[keyID] = conn
}

// RemoveConnection removes a connection associated with a specific user by its unique ID.
// It takes user of type string and id of type uuid.UUID.
// It does not return any value but safely does nothing if the user or connection ID does not exist.
func (cm *ConnManager) RemoveConnection(keyID string, id uuid.UUID) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if conn, ok := cm.conns[keyID]; ok && conn.ID() == id {
		_ = conn.Close()
		delete(cm.conns, keyID)
	}
}

// RequestConnection attempts to establish a new connection for the specified user.
// It takes ctx of type context.Context and userID of type string.
// It returns a channel of type net.Conn to receive the connection or an error if the operation fails.
// It returns an error if no connections are available for the user, the user does not exist, or a command fails to send.
func (cm *ConnManager) RequestConnection(ctx context.Context, keyID string) (*core.ConnReq, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	conn, ok := cm.conns[keyID]
	if !ok {
		return nil, fmt.Errorf("no connections for user %s", keyID)
	}

	r := &connRequest{
		ctx: ctx,
		ch:  make(chan *core.ClientConn, 1),
	}

	req, err := conn.RequestConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to send connect command: %w", err)
	}

	cm.requests[req.ID()] = r

	return req, nil
}

// ResolveRequest resolves a pending connection request by sending the provided connection to the request's channel.
// It takes an id parameter of type uuid.UUID and a conn parameter of type net.Conn.
// If the request is not found or its context is canceled, the connection is closed and no further actions are taken.
func (cm *ConnManager) ResolveRequest(id uuid.UUID, conn *core.ClientConn) {
	cm.mu.Lock()
	req, ok := cm.requests[id]
	delete(cm.requests, id)
	cm.mu.Unlock()

	if !ok {
		return
	}

	select {
	case req.ch <- conn:
	case <-req.ctx.Done():
		_ = conn.Close()
	}
}

// CancelRequest cancels a pending connection request by its unique ID.
// It takes an id parameter of type uuid.UUID.
// It does not return any values.
// If the ID is not found, the function does nothing.
func (cm *ConnManager) CancelRequest(id uuid.UUID) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	req, ok := cm.requests[id]
	if !ok {
		return
	}

	close(req.ch)
	delete(cm.requests, id)
}

// Close releases all resources managed by ConnManager and terminates active connections gracefully.
// It returns an error if at least one connection fails to close properly.
// It ensures thread-safety and cleans up all user connections and pending requests.
func (cm *ConnManager) Close() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	errs := make([]error, 0, len(cm.requests))

	for id, req := range cm.requests {
		close(req.ch)
		delete(cm.requests, id)
	}

	for _, userConn := range cm.conns {
		err := userConn.Close()
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to close connections: %w", errors.Join(errs...))
	}

	return nil
}
