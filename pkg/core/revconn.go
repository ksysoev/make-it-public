package core

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/ksysoev/revdial/proto"
)

type ServerConn interface {
	ID() uuid.UUID
	Close() error
	SendConnectCommand(id uuid.UUID) error
	State() proto.State
}

type RevConn struct {
	ServerConn
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewRevConn(ctx context.Context, conn ServerConn) (*RevConn, error) {
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		<-ctx.Done()

		_ = conn.Close()
	}()

	return &RevConn{
		ServerConn: conn,
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

func (r *RevConn) Context() context.Context {
	return r.ctx
}

func (r *RevConn) Close() error {
	r.cancel()
	r.wg.Wait()

	return nil
}
