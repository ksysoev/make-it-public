package conn

import (
	"context"
	"errors"
	"net"
	"os"
	"time"
)

const interval = 10 * time.Millisecond

type ContextConnNopCloser struct {
	net.Conn
	ctx    context.Context
	cancel context.CancelFunc
}

// NewContextConnNopCloser wraps a net.Conn with a context-aware layer and a no-op Close implementation.
// It creates a new ContextConnNopCloser that ensures read methods respect the provided context's cancellation.
// Accepts ctx the context to manage cancellation and conn the underlying network connection.
// Returns a ContextConnNopCloser that integrates context cancellation with the provided connection's lifecycle.
func NewContextConnNopCloser(ctx context.Context, conn net.Conn) *ContextConnNopCloser {
	ctx, cancel := context.WithCancel(ctx)

	return &ContextConnNopCloser{
		Conn:   conn,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Read reads up to len(p) bytes into p, respecting the context's deadline and cancellation.
// It returns the number of bytes read and an error if reading fails or the context is canceled.
func (c *ContextConnNopCloser) Read(p []byte) (int, error) {
	n := 0

	for c.ctx.Err() == nil {
		deadline := time.Now().Add(interval)

		ctxDeadline, ok := c.ctx.Deadline()

		if ok && ctxDeadline.Before(deadline) {
			deadline = ctxDeadline
		}

		if err := c.SetReadDeadline(deadline); err != nil {
			return 0, err
		}

		cn, err := c.Conn.Read(p[n:])
		n += cn

		switch {
		case err == nil:
			return n, nil
		case errors.Is(err, os.ErrDeadlineExceeded):
			if n == len(p) {
				return n, nil
			}

			continue
		default:
			return n, err
		}
	}

	return n, c.ctx.Err()
}

// Close cancels the context associated with the connection and releases its resources.
// It ensures cleanup of the context's state and prevents further operations that rely on the context.
// Returns an error only if any issues occur during the cancellation process, though typically nil.
func (c *ContextConnNopCloser) Close() error {
	c.cancel()

	return nil
}
