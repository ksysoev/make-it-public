package core

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

func NewContextConnNopCloser(ctx context.Context, conn net.Conn) *ContextConnNopCloser {
	ctx, cancel := context.WithCancel(ctx)
	return &ContextConnNopCloser{
		Conn:   conn,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (c *ContextConnNopCloser) Read(p []byte) (int, error) {
	if c.ctx.Err() != nil {
		return 0, c.ctx.Err()
	}

	deadline := time.Now().Add(interval)

	ctxDeadline, ok := c.ctx.Deadline()

	if ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}

	if err := c.Conn.SetReadDeadline(deadline); err != nil {
		return 0, err
	}

	n, err := c.Conn.Read(p)
	switch {
	case err == nil:
		return n, nil
	case errors.Is(err, os.ErrDeadlineExceeded):
		return n, nil
	default:
		return n, err
	}
}

func (c *ContextConnNopCloser) Close() error {
	c.cancel()

	return nil
}
