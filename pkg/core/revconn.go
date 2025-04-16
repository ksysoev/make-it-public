package core

import (
	"context"
	"fmt"
	"net"

	"github.com/google/uuid"
	"github.com/ksysoev/revdial/proto"
)

// serverConn defines the interface for managing a server connection, including control commands and state retrieval.
// ID returns the unique identifier of the server connection.
// Close terminates the server connection and releases associated resources. It may return an error if the operation fails.
// SendConnectCommand sends a connection command to the server with the given ID. It returns an error if the operation fails.
// State retrieves the current protocol state of the server connection.
type serverConn interface {
	ID() uuid.UUID
	Close() error
	SendConnectCommand(id uuid.UUID) error
	State() proto.State
}

// ServConn represents a managed server connection with support for context cancellation.
// It embeds a serverConn instance to handle low-level connection operations and state management.
// ServConn ensures proper context handling and provides methods to interact with and manage the connection's lifecycle.
type ServConn struct {
	conn   serverConn
	ctx    context.Context
	cancel context.CancelFunc
}

// NewServerConn creates a new managed server connection with context support.
// It initializes the ServConn with the provided context and serverConn interface.
// Returns a pointer to ServConn, which manages the server connection and supports context cancellation.
func NewServerConn(ctx context.Context, conn serverConn) *ServConn {
	ctx, cancel := context.WithCancel(ctx)

	return &ServConn{
		conn:   conn,
		ctx:    ctx,
		cancel: cancel,
	}
}

// ID retrieves the unique identifier (UUID) of the server connection wrapper instance. It ensures thread-safe access.
func (r *ServConn) ID() uuid.UUID {
	return r.conn.ID()
}

// Context retrieves the context associated with the ServConn instance.
// It provides context for managing lifecycle, cancellation, and timeout of the connection.
// Returns the context.Context instance associated with the server connection.
func (r *ServConn) Context() context.Context {
	return r.ctx
}

// Close releases the server connection and cancels the associated context to free resources of the ServConn instance.
// It returns an error if the underlying connection cannot be successfully closed.
func (r *ServConn) Close() error {
	defer r.cancel()

	return r.conn.Close()
}

// RequestConnection initiates a new connection request by issuing a connect command to the server.
// It ensures the server is in a registered state before proceeding.
// Returns a pointer to ConnReq containing the connection request details and an error if the server is not connected or if the command fails to send.
func (r *ServConn) RequestConnection() (*ConnReq, error) {
	if r.conn.State() != proto.StateRegistered {
		return nil, fmt.Errorf("server is not connected")
	}

	req := NewConnReq(r.Context())
	if err := r.conn.SendConnectCommand(req.ID()); err != nil {
		return nil, fmt.Errorf("failed to send connect command: %w", err)
	}

	return req, nil
}

// ClientConn extends net.Conn to include context cancellation support.
// It embeds net.Conn for standard connection operations and adds a context and cancellation mechanism.
// The context can be used to manage the connection lifecycle, including cancellation in long-running operations.
type ClientConn struct {
	net.Conn
	ctx    context.Context
	cancel context.CancelFunc
}

// NewClientConn creates a new ClientConn instance with context cancellation support.
// It initializes the ClientConn with the provided context and a net.Conn object.
// Returns a pointer to ClientConn managing the connection lifecycle, including context cancellation.
func NewClientConn(ctx context.Context, conn net.Conn) *ClientConn {
	ctx, cancel := context.WithCancel(ctx)

	return &ClientConn{
		Conn:   conn,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Context retrieves the context associated with the ClientConn instance.
// It provides lifecycle management capabilities, including support for cancellation and timeout.
// Returns the context.Context instance tied to the connection.
func (c *ClientConn) Context() context.Context {
	return c.ctx
}

// Close terminates the connection and cancels the associated context.
// It ensures that resources tied to the connection are released properly.
// Returns an error only if closing the underlying connection fails.
func (c *ClientConn) Close() error {
	defer c.cancel()

	return c.Conn.Close()
}
