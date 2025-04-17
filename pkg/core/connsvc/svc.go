package connsvc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"syscall"

	"github.com/google/uuid"
	"github.com/ksysoev/make-it-public/pkg/core"
	"github.com/ksysoev/revdial/proto"
	"golang.org/x/sync/errgroup"
)

type AuthRepo interface {
	Verify(user, pass string) bool
}

type ConnManager interface {
	RequestConnection(ctx context.Context, userID string) (core.ConnReq, error)
	AddConnection(user string, conn core.ServConn)
	ResolveRequest(id uuid.UUID, conn net.Conn)
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

func (s *Service) HandleReverseConn(ctx context.Context, conn net.Conn) error {
	var connUser string

	slog.DebugContext(ctx, "new connection", slog.Any("remote", conn.RemoteAddr()))
	defer slog.DebugContext(ctx, "closing connection", slog.Any("remote", conn.RemoteAddr()))

	servConn := proto.NewServer(conn, proto.WithUserPassAuth(func(user, pass string) bool {
		if s.auth.Verify(user, pass) {
			connUser = user
			return true
		}

		return false
	}))

	if err := servConn.Process(); err != nil {
		return fmt.Errorf("failed to process connection: %w", err)
	}

	switch servConn.State() {
	case proto.StateRegistered:
		srvConn := core.NewServerConn(ctx, servConn)

		s.connmng.AddConnection(connUser, srvConn)
		slog.DebugContext(ctx, "control connection established", slog.Any("remote", conn.RemoteAddr()))

		// TODO: currently we don't have possibility to identify closed connection
		// when we add ping command to the protocol, we should add here for loop with sending ping command for checking state of the connection

		<-srvConn.Context().Done()

		return nil
	case proto.StateBound:
		notifier := core.NewCloseNotifier(conn)

		s.connmng.ResolveRequest(servConn.ID(), notifier)
		slog.DebugContext(ctx, "bound connection established", slog.Any("remote", conn.RemoteAddr()), slog.Any("id", servConn.ID()))

		notifier.WaitClose(ctx)

		return nil
	default:
		return fmt.Errorf("unexpected state while handling incomming connection: %d", servConn.State())
	}
}

func (s *Service) HandleHTTPConnection(ctx context.Context, userID string, conn net.Conn, write func(net.Conn) error) error {
	slog.DebugContext(ctx, "new HTTP connection", slog.Any("remote", conn.RemoteAddr()))
	defer slog.DebugContext(ctx, "closing HTTP connection", slog.Any("remote", conn.RemoteAddr()))

	req, err := s.connmng.RequestConnection(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to request connection: %w", core.ErrFailedToConnect)
	}

	revConn, err := req.WaitConn(ctx)
	if err != nil {
		s.connmng.CancelRequest(req.ID())
		return fmt.Errorf("connection request failed: %w", core.ErrFailedToConnect)
	}

	slog.DebugContext(ctx, "connection received", slog.Any("remote", conn.RemoteAddr()))

	// Write initial request data
	if err := write(revConn); err != nil {
		slog.DebugContext(ctx, "failed to write initial request", slog.Any("error", err))

		return fmt.Errorf("failed to write initial request: %w", core.ErrFailedToConnect)
	}

	// Create error group for managing both copy operations
	eg, ctx := errgroup.WithContext(ctx)
	cliConn := core.NewContextConnNopCloser(ctx, conn)

	eg.Go(pipeConn(cliConn, revConn))
	eg.Go(pipeConn(revConn, cliConn))
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
				return core.ErrFailedToConnect
			}

			return io.EOF
		case err != nil:
			return fmt.Errorf("error copying from reverse connection: %w", err)
		}

		return io.EOF
	}
}
