package conn

import (
	"context"

	"github.com/google/uuid"
)

type ServConn interface {
	ID() uuid.UUID
	Context() context.Context
	Close() error
	RequestConnection() (Request, error)
}
