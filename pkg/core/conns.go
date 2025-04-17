package core

import (
	"context"
	"fmt"
	"net"

	"github.com/google/uuid"
	"github.com/ksysoev/revdial/proto"
)

type ServConn interface {
	ID() uuid.UUID
	Context() context.Context
	Close() error
	RequestConnection() (*connReq, error)
}

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

// servConn represents a managed server connection with support for context cancellation.
// It embeds a serverConn instance to handle low-level connection operations and state management.
// servConn ensures proper context handling and provides methods to interact with and manage the connection's lifecycle.
type servConn struct {
	conn   serverConn
	ctx    context.Context
	cancel context.CancelFunc
}

// NewServerConn creates a new managed server connection with context support.
// It initializes the servConn with the provided context and serverConn interface.
// Returns a pointer to servConn, which manages the server connection and supports context cancellation.
func NewServerConn(ctx context.Context, conn serverConn) *servConn {
	ctx, cancel := context.WithCancel(ctx)

	return &servConn{
		conn:   conn,
		ctx:    ctx,
		cancel: cancel,
	}
}

// ID retrieves the unique identifier (UUID) of the server connection wrapper instance. It ensures thread-safe access.
func (r *servConn) ID() uuid.UUID {
	return r.conn.ID()
}

// Context retrieves the context associated with the servConn instance.
// It provides context for managing lifecycle, cancellation, and timeout of the connection.
// Returns the context.Context instance associated with the server connection.
func (r *servConn) Context() context.Context {
	return r.ctx
}

// Close releases the server connection and cancels the associated context to free resources of the servConn instance.
// It returns an error if the underlying connection cannot be successfully closed.
func (r *servConn) Close() error {
	defer r.cancel()

	return r.conn.Close()
}

// RequestConnection initiates a new connection request by issuing a connect command to the server.
// It ensures the server is in a registered state before proceeding.
// Returns a pointer to connReq containing the connection request details and an error if the server is not connected or if the command fails to send.
func (r *servConn) RequestConnection() (*connReq, error) {
	if r.conn.State() != proto.StateRegistered {
		return nil, fmt.Errorf("server is not connected")
	}

	req := NewConnReq(r.Context())
	if err := r.conn.SendConnectCommand(req.ID()); err != nil {
		return nil, fmt.Errorf("failed to send connect command: %w", err)
	}

	return req, nil
}

// CloseNotifier is a type that wraps a network connection and provides a mechanism to signal when the connection is closed.
type CloseNotifier struct {
	net.Conn
	done chan struct{}
}

// NewCloseNotifier creates and returns a CloseNotifier wrapping the given network connection.
// It initializes a channel to signal when the connection is closed.
func NewCloseNotifier(conn net.Conn) *CloseNotifier {
	return &CloseNotifier{
		Conn: conn,
		done: make(chan struct{}),
	}
}

// WaitClose blocks until the CloseNotifier is closed or the provided context is canceled.
// It listens for the closure signal or context cancellation, whichever occurs first.
func (c *CloseNotifier) WaitClose(ctx context.Context) {
	select {
	case <-c.done:
	case <-ctx.Done():
	}
}

// Close terminates the underlying connection and signals closure via the done channel.
// It ensures the done channel is closed after invoking the connection's Close method.
// Returns an error if closing the connection fails.
func (c *CloseNotifier) Close() error {
	defer close(c.done)

	return c.Conn.Close()
}
