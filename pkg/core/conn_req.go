package core

import (
	"context"
	"fmt"
	"net"

	"github.com/google/uuid"
)

type ConnReq struct {
	ctx context.Context
	ch  chan net.Conn
	id  uuid.UUID
}

func NewConnReq(ctx context.Context) *ConnReq {
	return &ConnReq{
		id:  uuid.New(),
		ch:  make(chan net.Conn),
		ctx: ctx,
	}
}

func (r *ConnReq) ID() uuid.UUID {
	return r.id
}

func (r *ConnReq) ParentContext() context.Context {
	return r.ctx
}

func (r *ConnReq) WaitConn(ctx context.Context) (net.Conn, error) {
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

func (r *ConnReq) SendConn(ctx context.Context, conn net.Conn) {
	select {
	case <-ctx.Done():
		return
	case <-r.ctx.Done():
		return
	case r.ch <- conn:
	}
}

func (r *ConnReq) Cancel() {
	close(r.ch)
}
