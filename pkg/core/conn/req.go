package conn

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// Request represents an interface for managing connection requests and their lifecycle.
// It provides methods to retrieve a unique identifier, access associated contexts, wait for a response, and handle cancellation.
type Request interface {
	ID() uuid.UUID
	ParentContext() context.Context
	WaitConn(ctx context.Context) (WithWriteCloser, error)
	SendConn(ctx context.Context, conn WithWriteCloser)
	Cancel()
}

// request represents a connection request with a unique identifier, channel for delivering the connection, and context for cancellation.
type request struct {
	ctx context.Context
	ch  chan WithWriteCloser
	id  uuid.UUID
}

// newRequest creates a new request instance with a unique identifier, channel for delivering connections, and a context.
// It ensures the request is initialized with a provided parent context to manage cancellation or timeouts.
// Returns a pointer to the created request.
func newRequest(ctx context.Context) *request {
	return &request{
		id:  uuid.New(),
		ch:  make(chan WithWriteCloser),
		ctx: ctx,
	}
}

// ID retrieves the unique identifier (UUID) of the connection request.
func (r *request) ID() uuid.UUID {
	return r.id
}

// ParentContext retrieves the parent context associated with the connection request.
// It allows callers to observe cancellation or manage lifetimes using the parent's context.
func (r *request) ParentContext() context.Context {
	return r.ctx
}

// WaitConn waits for a network connection to be delivered through the request's channel or observes context cancellations.
// It returns the established net.Conn if successful or an error if the provided context, parent context, or request is canceled.
func (r *request) WaitConn(ctx context.Context) (WithWriteCloser, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-r.ctx.Done():
		return nil, fmt.Errorf("parent context is canceled")
	case conn, ok := <-r.ch:
		if !ok {
			return nil, fmt.Errorf("request is canceled")
		}

		return conn, nil
	}
}

// SendConn delivers the provided connection to the request's channel, allowing it to be accessed by a waiting operation.
// It returns immediately if the provided context or the parent context is done, ensuring no blocking occurs.
// ctx represents the context to observe for cancellation or deadlines.
// conn represents the network connection to be sent.
func (r *request) SendConn(ctx context.Context, conn WithWriteCloser) {
	select {
	case <-ctx.Done():
		return
	case <-r.ctx.Done():
		return
	case r.ch <- conn:
	}
}

// Cancel closes the connection request's channel to signal that the request is canceled. It ensures no further connections are delivered.
func (r *request) Cancel() {
	close(r.ch)
}
