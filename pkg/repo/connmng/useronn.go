package connmng

import (
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// UserConnections manages a thread-safe collection of ServerConn objects for a user.
// It maintains the connection pool, ensures proper concurrency control, and supports round-robin connection retrieval.
type UserConnections struct {
	connections []ServerConn
	current     int
	mu          sync.RWMutex
}

// NewUserConnections initializes and returns a new instance of UserConnections.
// It does not take any parameters.
// It returns a pointer to a UserConnections struct with an empty connections pool.
func NewUserConnections() *UserConnections {
	return &UserConnections{
		connections: make([]ServerConn, 0),
	}
}

// AddConnection adds a server connection to the user's connections pool.
// It takes a user parameter of type string and a conn parameter of type *proto.Server.
// If a connection with the same ID already exists, it closes the old connection and replaces it with the new one.
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

// RemoveConnection removes a connection from the user's pool based on its unique ID.
// It takes a user parameter of type string and an id parameter of type uuid.UUID.
// If there is no connection associated with the specified ID, it does nothing.
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

// GetConn retrieves the next available ServerConn from the UserConnections pool in a round-robin fashion.
// It does not take any parameters.
// It returns a ServerConn or nil if the pool is empty.
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

// Close releases all active connections in UserConnections and ensures proper cleanup.
// It iterates through all connections, attempts to close each one, and clears the list of connections.
// It returns an error if any of the connections fail to close.
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
