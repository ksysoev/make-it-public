package core

import (
	"context"
	"net"

	"github.com/google/uuid"
)

type ConnReq interface {
	ID() uuid.UUID
	ParentContext() context.Context
	WaitConn(ctx context.Context) (net.Conn, error)
	SendConn(ctx context.Context, conn net.Conn)
	Cancel()
}

type ServConn interface {
	ID() uuid.UUID
	Context() context.Context
	Close() error
	RequestConnection() (ConnReq, error)
}
