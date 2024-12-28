package connmng

import (
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

type UserConnections struct {
	connections []ServerConn
	current     int
	mu          sync.RWMutex
}

type ConnManager struct {
	users map[string]*UserConnections
}
