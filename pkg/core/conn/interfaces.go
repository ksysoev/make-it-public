package conn

import (
	"context"
	"net"

	"github.com/google/uuid"
)

type Req interface {
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
	RequestConnection() (Req, error)
}
