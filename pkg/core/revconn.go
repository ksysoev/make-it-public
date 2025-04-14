package core

import (
	"context"
	"net"

	"github.com/google/uuid"
	"github.com/ksysoev/revdial/proto"
)

type serverConn interface {
	ID() uuid.UUID
	Close() error
	SendConnectCommand(id uuid.UUID) error
	State() proto.State
}

type ServConn struct {
	serverConn
	ctx    context.Context
	cancel context.CancelFunc
}

func NewServerConn(ctx context.Context, conn serverConn) *ServConn {
	ctx, cancel := context.WithCancel(ctx)

	return &ServConn{
		serverConn: conn,
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (r *ServConn) Context() context.Context {
	return r.ctx
}

func (r *ServConn) Close() error {
	defer r.cancel()

	return r.serverConn.Close()
}

type ClientConn struct {
	net.Conn
	ctx    context.Context
	cancel context.CancelFunc
}

func NewClientConn(ctx context.Context, conn net.Conn) *ClientConn {
	ctx, cancel := context.WithCancel(ctx)

	return &ClientConn{
		Conn:   conn,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (c *ClientConn) Context() context.Context {
	return c.ctx
}

func (c *ClientConn) Close() error {
	defer c.cancel()

	return c.Conn.Close()
}
