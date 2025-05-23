package core

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/ksysoev/make-it-public/pkg/core/conn"
	"github.com/ksysoev/make-it-public/pkg/core/token"
)

type ControlConn interface {
	ID() uuid.UUID
	Context() context.Context
	Close() error
	RequestConnection() (conn.Request, error)
}

type AuthRepo interface {
	Verify(ctx context.Context, keyID, secret string) (bool, error)
	GenerateToken(ctx context.Context, keyID string, ttl time.Duration) (*token.Token, error)
	DeleteToken(ctx context.Context, tokenID string) error
}

type ConnManager interface {
	RequestConnection(ctx context.Context, keyID string) (conn.Request, error)
	AddConnection(keyID string, conn ControlConn)
	ResolveRequest(id uuid.UUID, conn net.Conn)
	RemoveConnection(keyID string, id uuid.UUID)
	CancelRequest(id uuid.UUID)
}

type Service struct {
	connmng           ConnManager
	auth              AuthRepo
	endpointGenerator func(string) (string, error)
}

func New(connmng ConnManager, auth AuthRepo) *Service {
	return &Service{
		connmng: connmng,
		auth:    auth,
		endpointGenerator: func(_ string) (string, error) {
			return "", fmt.Errorf("endpoint generator is not set")
		},
	}
}

func (s *Service) SetEndpointGenerator(generator func(string) (string, error)) {
	s.endpointGenerator = generator
}
