package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/ksysoev/make-it-public/pkg/core/conn"
	"github.com/ksysoev/revdial/proto"
	"golang.org/x/sync/errgroup"
)

type ControlConn interface {
	ID() uuid.UUID
	Context() context.Context
	Close() error
	RequestConnection() (conn.Request, error)
}

type AuthRepo interface {
	Verify(ctx context.Context, keyID, secret string) (bool, error)
}

type ConnManager interface {
	RequestConnection(ctx context.Context, userID string) (conn.Request, error)
	AddConnection(user string, conn ControlConn)
	ResolveRequest(id uuid.UUID, conn net.Conn)
	RemoveConnection(user string, id uuid.UUID)
	CancelRequest(id uuid.UUID)
}

type Service struct {
	connmng ConnManager
	auth    AuthRepo
}

func New(connmng ConnManager, auth AuthRepo) *Service {
	return &Service{
		connmng: connmng,
		auth:    auth,
	}
}

func (s *Service) HandleReverseConn(ctx context.Context, revConn net.Conn) error {
	var connKeyID string

	slog.DebugContext(ctx, "new connection", slog.Any("remote", revConn.RemoteAddr()))
	defer slog.DebugContext(ctx, "closing connection", slog.Any("remote", revConn.RemoteAddr()))

	servConn := proto.NewServer(revConn, proto.WithUserPassAuth(func(keyID, secret string) bool {
		valid, err := s.auth.Verify(ctx, keyID, secret)
		if err == nil && valid {
			connKeyID = keyID
			return true
		} else if err != nil {
			slog.ErrorContext(ctx, "failed to verify user", slog.Any("error", err))
		}

		return false
	}))

	if err := servConn.Process(); err != nil {
		return fmt.Errorf("failed to process connection: %w", err)
	}

	switch servConn.State() {
	case proto.StateRegistered:
		srvConn := conn.NewServerConn(ctx, servConn)

		s.connmng.AddConnection(connKeyID, srvConn)

		defer s.connmng.RemoveConnection(connKeyID, srvConn.ID())

		slog.DebugContext(ctx, "control connection established", slog.Any("remote", revConn.RemoteAddr()))

		for {
			select {
			case <-srvConn.Context().Done():
				return nil
			case <-time.After(200 * time.Millisecond):
			}

			err := srvConn.Ping()
			if err != nil {
				slog.DebugContext(ctx, "ping failed", slog.Any("error", err))
				return fmt.Errorf("ping failed: %w", err)
			}
		}
	case proto.StateBound:
		notifier := conn.NewCloseNotifier(revConn)

		s.connmng.ResolveRequest(servConn.ID(), notifier)
		slog.DebugContext(ctx, "bound connection established", slog.Any("remote", revConn.RemoteAddr()), slog.Any("id", servConn.ID()))

		notifier.WaitClose(ctx)

		return nil
	default:
		return fmt.Errorf("unexpected state while handling incomming connection: %d", servConn.State())
	}
}

func (s *Service) HandleHTTPConnection(ctx context.Context, userID string, cliConn net.Conn, write func(net.Conn) error) error {
	slog.DebugContext(ctx, "new HTTP connection", slog.Any("remote", cliConn.RemoteAddr()))
	defer slog.DebugContext(ctx, "closing HTTP connection", slog.Any("remote", cliConn.RemoteAddr()))

	req, err := s.connmng.RequestConnection(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to request connection: %w", ErrFailedToConnect)
	}

	revConn, err := req.WaitConn(ctx)
	if err != nil {
		s.connmng.CancelRequest(req.ID())
		return fmt.Errorf("connection request failed: %w", ErrFailedToConnect)
	}

	slog.DebugContext(ctx, "connection received", slog.Any("remote", cliConn.RemoteAddr()))

	// Write initial request data
	if err := write(revConn); err != nil {
		slog.DebugContext(ctx, "failed to write initial request", slog.Any("error", err))

		return fmt.Errorf("failed to write initial request: %w", ErrFailedToConnect)
	}

	// Create error group for managing both copy operations
	eg, ctx := errgroup.WithContext(ctx)
	connNopCloser := conn.NewContextConnNopCloser(ctx, cliConn)

	eg.Go(pipeConn(connNopCloser, revConn))
	eg.Go(pipeConn(revConn, connNopCloser))
	eg.Go(func() error {
		select {
		case <-ctx.Done():
		case <-req.ParentContext().Done(): // Pare
		}

		return revConn.Close()
	})

	if err := eg.Wait(); !errors.Is(err, io.EOF) {
		return err
	}

	return nil
}

// pipeConn manages bidirectional copying of data between a source reader and a destination writer.
// It reads from src and writes to dst, handling specific network-related errors gracefully.
// Returns a function that performs the copy operation, returning io.EOF on successful completion or a detailed error on failure.
func pipeConn(src io.Reader, dst io.Writer) func() error {
	return func() error {
		n, err := io.Copy(dst, src)

		switch {
		case errors.Is(err, net.ErrClosed), errors.Is(err, syscall.ECONNRESET):
			if n == 0 {
				return ErrFailedToConnect
			}

			return io.EOF
		case err != nil:
			return fmt.Errorf("error copying from reverse connection: %w", err)
		}

		return io.EOF
	}
}
