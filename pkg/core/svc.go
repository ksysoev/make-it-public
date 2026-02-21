package core

import (
	"context"
	"fmt"

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
	Verify(ctx context.Context, keyID, secret string) (bool, token.TokenType, error)
	SaveToken(ctx context.Context, t *token.Token) error
	DeleteToken(ctx context.Context, tokenID string) error
	IsKeyExists(ctx context.Context, keyID string) (bool, error)
	CheckHealth(ctx context.Context) error
}

type ConnManager interface {
	RequestConnection(ctx context.Context, keyID string) (conn.Request, error)
	AddConnection(keyID string, conn ControlConn)
	ResolveRequest(id uuid.UUID, conn conn.WithWriteCloser)
	RemoveConnection(keyID string, id uuid.UUID)
	CancelRequest(id uuid.UUID)
}

// TCPEndpointAllocator dynamically allocates and releases TCP listeners for
// individual MIT clients that authenticate with a TCP token.
// Allocate starts a TCP listener and returns the public endpoint (host:port).
// Release stops the listener and frees the port back to the pool.
type TCPEndpointAllocator interface {
	Allocate(ctx context.Context, keyID string) (string, error)
	Release(keyID string)
}

type Service struct {
	endpointGenerator    func(string) (string, error)
	tcpEndpointAllocator TCPEndpointAllocator
	webConnMng           ConnManager
	tcpConnMng           ConnManager
	auth                 AuthRepo
}

// New initializes and returns a new Service instance with the provided ConnManagers and AuthRepo.
// It assigns a default endpoint generator function that returns an error if invoked.
// webConnMng manages web/HTTP connection-related operations.
// tcpConnMng manages TCP connection-related operations.
// auth handles authentication-related operations.
func New(webConnMng, tcpConnMng ConnManager, auth AuthRepo) *Service {
	return &Service{
		webConnMng: webConnMng,
		tcpConnMng: tcpConnMng,
		auth:       auth,
		endpointGenerator: func(_ string) (string, error) {
			return "", fmt.Errorf("endpoint generator is not set")
		},
		tcpEndpointAllocator: noopTCPEndpointAllocator{},
	}
}

// SetEndpointGenerator sets a custom function to generate endpoints dynamically based on a provided key.
// It updates the internal endpoint generation logic with the provided function.
// Accepts generator as a function taking a string and returning a string as the generated endpoint and an error.
// Returns no values, but any errors from the generator function should be handled internally by its caller.
func (s *Service) SetEndpointGenerator(generator func(string) (string, error)) {
	s.endpointGenerator = generator
}

// SetTCPEndpointAllocator sets the allocator used to create per-keyID TCP listeners.
// It is called by the TCP edge server during initialisation.
func (s *Service) SetTCPEndpointAllocator(allocator TCPEndpointAllocator) {
	s.tcpEndpointAllocator = allocator
}

func (s *Service) CheckHealth(ctx context.Context) error {
	return s.auth.CheckHealth(ctx)
}

// noopTCPEndpointAllocator is the default allocator used when no TCP edge
// server has been wired in.  It returns an error on every Allocate call so that
// TCP tokens are rejected cleanly rather than silently misbehaving.
type noopTCPEndpointAllocator struct{}

func (noopTCPEndpointAllocator) Allocate(_ context.Context, keyID string) (string, error) {
	return "", fmt.Errorf("TCP endpoint allocator is not configured (keyID=%s)", keyID)
}

func (noopTCPEndpointAllocator) Release(_ string) {}
