package connmng

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/google/uuid"
	"github.com/ksysoev/revdial/proto"
)

type connRequest struct {
	ctx context.Context
	ch  chan net.Conn
}

type ServerConn interface {
	ID() uuid.UUID
	Close() error
	SendConnectCommand(id uuid.UUID) error
	State() proto.State
}

type ConnManager struct {
	users    map[string]*UserConnections
	requests map[uuid.UUID]*connRequest
	mu       sync.RWMutex
}

func New() *ConnManager {
	return &ConnManager{
		users:    make(map[string]*UserConnections),
		requests: make(map[uuid.UUID]*connRequest),
	}
}

func (cm *ConnManager) AddConnection(user string, conn ServerConn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	userConn, ok := cm.users[user]
	if !ok {
		userConn = NewUserConnections()
		cm.users[user] = userConn
	}

	userConn.AddConnection(conn)
}

func (cm *ConnManager) RemoveConnection(user string, id uuid.UUID) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	userConn, ok := cm.users[user]
	if !ok {
		return
	}

	userConn.RemoveConnection(id)
}

func (cm *ConnManager) RequestConnection(ctx context.Context, userID string) (chan net.Conn, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	userConn, ok := cm.users[userID]
	if !ok {
		return nil, fmt.Errorf("No connections for user %s", userID)
	}

	cliConn := userConn.GetConn()
	if cliConn == nil {
		return nil, fmt.Errorf("No connections for user %s", userID)
	}

	id := uuid.New()
	req := &connRequest{
		ctx: ctx,
		ch:  make(chan net.Conn, 1),
	}

	cm.requests[id] = req

	if err := cliConn.SendConnectCommand(id); err != nil {
		delete(cm.requests, id)

		return nil, fmt.Errorf("failed to send connect command: %w", err)
	}

	return req.ch, nil
}

func (cm *ConnManager) ResolveRequest(id uuid.UUID, conn net.Conn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	req, ok := cm.requests[id]
	if !ok {
		return
	}

	select {
	case req.ch <- conn:
	case <-req.ctx.Done():
		_ = conn.Close()
	}

	delete(cm.requests, id)
}

func (cm *ConnManager) CancelRequest(id uuid.UUID) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	req, ok := cm.requests[id]
	if !ok {
		return
	}

	close(req.ch)
	delete(cm.requests, id)
}

func (cm *ConnManager) Close() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	errs := make([]error, 0, len(cm.requests))

	for id, req := range cm.requests {
		close(req.ch)
		delete(cm.requests, id)
	}

	for _, userConn := range cm.users {
		err := userConn.Close()
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to close connections: %w", errs)
	}

	return nil
}
