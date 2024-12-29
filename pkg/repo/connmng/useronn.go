package connmng

import (
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

type UserConnections struct {
	connections []ServerConn
	current     int
	mu          sync.RWMutex
}

func NewUserConnections() *UserConnections {
	return &UserConnections{
		connections: make([]ServerConn, 0),
	}
}

func (uc *UserConnections) AddConnection(serv ServerConn) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	for i, conn := range uc.connections {
		if conn.ID() == serv.ID() {
			_ = conn.Close()
			uc.connections[i] = serv

			return
		}
	}

	uc.connections = append(uc.connections, serv)
}

func (uc *UserConnections) RemoveConnection(id uuid.UUID) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	for i, conn := range uc.connections {
		if conn.ID() == id {
			_ = conn.Close()

			uc.connections = append(uc.connections[:i], uc.connections[i+1:]...)

			return
		}
	}
}

func (uc *UserConnections) GetConn() ServerConn {
	uc.mu.RLock()
	defer uc.mu.RUnlock()

	if len(uc.connections) == 0 {
		return nil
	}

	conn := uc.connections[uc.current]
	uc.current = (uc.current + 1) % len(uc.connections)

	return conn
}

func (uc *UserConnections) Close() error {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	errs := make([]error, 0, len(uc.connections))

	for _, conn := range uc.connections {
		err := conn.Close()
		if err != nil {
			errs = append(errs, err)
		}
	}

	uc.connections = uc.connections[:0]

	if len(errs) > 0 {
		return fmt.Errorf("failed to close connections: %w", errors.Join(errs...))
	}

	return nil
}
