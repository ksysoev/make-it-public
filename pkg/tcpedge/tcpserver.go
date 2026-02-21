package tcpedge

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"github.com/ksysoev/make-it-public/pkg/core"
)

// ErrKeyIDAlreadyAllocated is returned when Allocate is called for a keyID that
// already has an active listener.
var ErrKeyIDAlreadyAllocated = errors.New("keyID already has an active TCP listener")

// ConnService is the subset of core.Service required by the TCP edge server.
type ConnService interface {
	HandleTCPConnection(ctx context.Context, keyID string, conn net.Conn, clientIP string) error
	SetTCPEndpointAllocator(allocator core.TCPEndpointAllocator)
}

// activeListener tracks a running per-keyID TCP listener and its goroutines.
type activeListener struct {
	cancel   context.CancelFunc
	listener net.Listener
	wg       sync.WaitGroup
	port     int
}

// TCPServer dynamically allocates TCP listeners for each connected MIT client
// that authenticated with a TCP token.  It implements core.TCPEndpointAllocator.
type TCPServer struct {
	connService ConnService
	portPool    *portPool
	listeners   map[string]*activeListener
	config      Config
	mu          sync.RWMutex
}

// New validates cfg, creates a TCPServer, and injects it as the
// TCPEndpointAllocator into connService.
func New(cfg Config, connService ConnService) (*TCPServer, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid TCP edge config: %w", err)
	}

	s := &TCPServer{
		connService: connService,
		portPool:    newPortPool(cfg.PortRange.Min, cfg.PortRange.Max),
		listeners:   make(map[string]*activeListener),
		config:      cfg,
	}

	connService.SetTCPEndpointAllocator(s)

	return s, nil
}

// Run blocks until ctx is cancelled, then closes all active per-keyID listeners.
func (s *TCPServer) Run(ctx context.Context) error {
	<-ctx.Done()
	s.closeAllListeners()

	return nil
}

// Allocate creates a TCP listener for keyID and returns the public endpoint
// string in the form "host:port".  The listener will accept end-user connections
// and route them through the tunnel.
//
// Allocate is called by core.Service when a TCP MIT client completes
// authentication (StateRegistered).  It must be balanced by a call to Release.
func (s *TCPServer) Allocate(ctx context.Context, keyID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.listeners[keyID]; exists {
		return "", fmt.Errorf("allocate keyID=%s: %w", keyID, ErrKeyIDAlreadyAllocated)
	}

	port, err := s.portPool.Allocate()
	if err != nil {
		return "", fmt.Errorf("allocate port for keyID=%s: %w", keyID, err)
	}

	addr := fmt.Sprintf("%s:%d", s.config.ListenHost, port)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		s.portPool.Release(port)
		return "", fmt.Errorf("listen on %s for keyID=%s: %w", addr, keyID, err)
	}

	listenerCtx, cancel := context.WithCancel(ctx)

	al := &activeListener{
		listener: ln,
		port:     port,
		cancel:   cancel,
	}

	s.listeners[keyID] = al

	al.wg.Add(1)

	go func() {
		defer al.wg.Done()

		s.acceptLoop(listenerCtx, al, keyID)
	}()

	endpoint := fmt.Sprintf("%s:%d", s.config.Public.Host, port)

	slog.InfoContext(ctx, "TCP listener allocated",
		slog.String("keyID", keyID),
		slog.String("endpoint", endpoint),
		slog.String("listen", addr))

	return endpoint, nil
}

// Release stops the listener associated with keyID and returns its port to the
// pool.  It is safe to call Release on a keyID that has already been released.
//
// Release is called by core.Service via a deferred call in HandleReverseConn so
// it executes when the MIT client disconnects.
func (s *TCPServer) Release(keyID string) {
	s.mu.Lock()

	al, exists := s.listeners[keyID]
	if !exists {
		s.mu.Unlock()
		return
	}

	delete(s.listeners, keyID)
	s.mu.Unlock()

	al.cancel()

	_ = al.listener.Close()

	al.wg.Wait()

	s.portPool.Release(al.port)

	slog.Info("TCP listener released", slog.String("keyID", keyID), slog.Int("port", al.port))
}

// acceptLoop runs the accept loop for a single per-keyID listener.
func (s *TCPServer) acceptLoop(ctx context.Context, al *activeListener, keyID string) {
	for {
		conn, err := al.listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return // normal shutdown via Release or server stop
			}

			slog.ErrorContext(ctx, "TCP accept error",
				slog.String("keyID", keyID),
				slog.Any("error", err))

			return
		}

		al.wg.Add(1)

		go func() {
			defer al.wg.Done()

			s.handleConn(ctx, keyID, conn)
		}()
	}
}

// handleConn routes a single end-user TCP connection through the tunnel.
func (s *TCPServer) handleConn(ctx context.Context, keyID string, conn net.Conn) {
	defer func() { _ = conn.Close() }()

	clientIP := ""
	if addr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		clientIP = addr.IP.String()
	}

	slog.DebugContext(ctx, "TCP end-user connection",
		slog.String("keyID", keyID),
		slog.String("clientIP", clientIP))

	if err := s.connService.HandleTCPConnection(ctx, keyID, conn, clientIP); err != nil {
		slog.DebugContext(ctx, "TCP connection closed",
			slog.String("keyID", keyID),
			slog.String("clientIP", clientIP),
			slog.Any("error", err))
	}
}

// closeAllListeners shuts down every active listener.  Called on server stop.
func (s *TCPServer) closeAllListeners() {
	s.mu.Lock()
	keys := make([]string, 0, len(s.listeners))

	for k := range s.listeners {
		keys = append(keys, k)
	}

	s.mu.Unlock()

	for _, keyID := range keys {
		s.Release(keyID)
	}
}
