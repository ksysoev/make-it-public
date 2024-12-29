package connmng

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/google/uuid"
	"github.com/ksysoev/revdial/proto"
)

type connRequest struct {
	ctx context.Context
	ch  chan net.Conn
}

type ServerConn interface {
	ID() uuid.UUID
	Close() error
	SendConnectCommand(id uuid.UUID) error
	State() proto.State
}

type ConnManager struct {
	users    map[string]*UserConnections
	requests map[uuid.UUID]*connRequest
	mu       sync.RWMutex
}

// New creates and returns a new instance of ConnManager.
// It does not take any parameters.
// It returns a pointer to a ConnManager with initialized internal maps for users and requests.
func New() *ConnManager {
	return &ConnManager{
		users:    make(map[string]*UserConnections),
		requests: make(map[uuid.UUID]*connRequest),
	}
}

// AddConnection adds a server connection to the user's connection pool.
// It takes a user parameter of type string and a conn parameter of type *proto.Server.
// It does not return any value and ensures thread-safe access.
func (cm *ConnManager) AddConnection(user string, conn *proto.Server) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	userConn, ok := cm.users[user]
	if !ok {
		userConn = NewUserConnections()
		cm.users[user] = userConn
	}

	userConn.AddConnection(conn)
}

// RemoveConnection removes a connection associated with a specific user by its unique ID.
// It takes user of type string and id of type uuid.UUID.
// It does not return any value but safely does nothing if the user or connection ID does not exist.
func (cm *ConnManager) RemoveConnection(user string, id uuid.UUID) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	userConn, ok := cm.users[user]
	if !ok {
		return
	}

	userConn.RemoveConnection(id)
}

// RequestConnection attempts to establish a new connection for the specified user.
// It takes ctx of type context.Context and userID of type string.
// It returns a channel of type net.Conn to receive the connection or an error if the operation fails.
// It returns an error if no connections are available for the user, the user does not exist, or a command fails to send.
func (cm *ConnManager) RequestConnection(ctx context.Context, userID string) (chan net.Conn, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	userConn, ok := cm.users[userID]
	if !ok {
		return nil, fmt.Errorf("no connections for user %s", userID)
	}

	cliConn := userConn.GetConn()
	if cliConn == nil {
		return nil, fmt.Errorf("no connections for user %s", userID)
	}

	id := uuid.New()
	req := &connRequest{
		ctx: ctx,
		ch:  make(chan net.Conn, 1),
	}

	cm.requests[id] = req

	if err := cliConn.SendConnectCommand(id); err != nil {
		delete(cm.requests, id)

		return nil, fmt.Errorf("failed to send connect command: %w", err)
	}

	return req.ch, nil
}

// ResolveRequest resolves a pending connection request by sending the provided connection to the request's channel.
// It takes an id parameter of type uuid.UUID and a conn parameter of type net.Conn.
// If the request is not found or its context is canceled, the connection is closed and no further actions are taken.
func (cm *ConnManager) ResolveRequest(id uuid.UUID, conn net.Conn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	req, ok := cm.requests[id]
	if !ok {
		return
	}

	select {
	case req.ch <- conn:
	case <-req.ctx.Done():
		_ = conn.Close()
	}

	delete(cm.requests, id)
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

	for _, userConn := range cm.users {
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
