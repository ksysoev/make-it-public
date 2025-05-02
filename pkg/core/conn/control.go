package conn

import (
	"context"
	"fmt"

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
	SendPingCommand() error
	SendCustomEvent(name string, data any) error
	State() proto.State
}

// ControlConn represents a managed server connection with support for context cancellation.
// It embeds a serverConn instance to handle low-level connection operations and state management.
// ControlConn ensures proper context handling and provides methods to interact with and manage the connection's lifecycle.
type ControlConn struct {
	conn   serverConn
	ctx    context.Context
	cancel context.CancelFunc
}

// NewServerConn creates a new managed server connection with context support.
// It initializes the ControlConn with the provided context and serverConn interface.
// Returns a pointer to ControlConn, which manages the server connection and supports context cancellation.
func NewServerConn(ctx context.Context, conn serverConn) *ControlConn {
	ctx, cancel := context.WithCancel(ctx)

	return &ControlConn{
		conn:   conn,
		ctx:    ctx,
		cancel: cancel,
	}
}

// ID retrieves the unique identifier (UUID) of the server connection wrapper instance. It ensures thread-safe access.
func (r *ControlConn) ID() uuid.UUID {
	return r.conn.ID()
}

// Context retrieves the context associated with the ControlConn instance.
// It provides context for managing lifecycle, cancellation, and timeout of the connection.
// Returns the context.Context instance associated with the server connection.
func (r *ControlConn) Context() context.Context {
	return r.ctx
}

// Close releases the server connection and cancels the associated context to free resources of the ControlConn instance.
// It returns an error if the underlying connection cannot be successfully closed.
func (r *ControlConn) Close() error {
	defer r.cancel()

	return r.conn.Close()
}

// RequestConnection initiates a new connection request by issuing a connect command to the server.
// It ensures the server is in a registered state before proceeding.
// Returns a pointer to request containing the connection request details and an error if the server is not connected or if the command fails to send.
func (r *ControlConn) RequestConnection() (Request, error) {
	req := newRequest(r.Context())
	if err := r.conn.SendConnectCommand(req.ID()); err != nil {
		return nil, fmt.Errorf("failed to send connect command: %w", err)
	}

	return req, nil
}

// Ping sends a ping command to the server to verify the connection's responsiveness.
// It returns an error if the ping command fails to send or encounters an issue.
func (r *ControlConn) Ping() error {
	if err := r.conn.SendPingCommand(); err != nil {
		return fmt.Errorf("failed to send ping command: %w", err)
	}

	return nil
}

func (r *ControlConn) SendUrlToConnectUpdatedEvent(url string) error {
	return r.conn.SendCustomEvent("urlToConnectUpdated", url)
}
