package conn

import (
	"context"
	"fmt"
	"net"
)

type WithWriteCloser interface {
	net.Conn
	CloseWrite() error
}

// CloseNotifier is a type that wraps a network connection and provides a mechanism to signal when the connection is closed.
type CloseNotifier struct {
	WithWriteCloser
	done chan struct{}
}

// NewCloseNotifier creates and returns a CloseNotifier wrapping the given network connection.
// It initializes a channel to signal when the connection is closed.
func NewCloseNotifier(conn net.Conn) (*CloseNotifier, error) {
	c, ok := conn.(WithWriteCloser)
	if !ok {
		return nil, fmt.Errorf("connection does not implement WithWriteCloser interface")
	}

	return &CloseNotifier{
		WithWriteCloser: c,
		done:            make(chan struct{}),
	}, nil
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

	return c.WithWriteCloser.Close()
}
