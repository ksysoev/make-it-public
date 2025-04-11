package connsvc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/google/uuid"
	"github.com/ksysoev/revdial/proto"
	"golang.org/x/sync/errgroup"
)

var ErrFailedToConnect = errors.New("failed to connect")

type AuthRepo interface {
	Verify(user, pass string) bool
}

type ConnManager interface {
	RequestConnection(ctx context.Context, userID string) (chan net.Conn, error)
	AddConnection(user string, conn *proto.Server)
	ResolveRequest(id uuid.UUID, conn net.Conn)
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
		s.connmng.AddConnection(connUser, servConn)
		slog.DebugContext(ctx, "control connection established", slog.Any("remote", conn.RemoteAddr()))
	case proto.StateBound:
		s.connmng.ResolveRequest(servConn.ID(), conn)
		slog.DebugContext(ctx, "bound connection established", slog.Any("remote", conn.RemoteAddr()), slog.Any("id", servConn.ID()))
	default:
		slog.ErrorContext(ctx, "unexpected state while handling incomming connection", slog.Any("state", servConn.State()))
	}

	<-ctx.Done()

	return nil
}

func (s *Service) HandleHTTPConnection(ctx context.Context, userID string, conn net.Conn, write func(net.Conn) error) error {
	slog.DebugContext(ctx, "new HTTP connection", slog.Any("remote", conn.RemoteAddr()))
	defer slog.DebugContext(ctx, "closing HTTP connection", slog.Any("remote", conn.RemoteAddr()))

	ch, err := s.connmng.RequestConnection(ctx, userID)
	if err != nil {
		return errors.Join(ErrFailedToConnect, err)
	}

	select {
	case <-ctx.Done():
		return errors.Join(ErrFailedToConnect, ctx.Err())
	case revConn, ok := <-ch:
		if !ok {
			return errors.Join(fmt.Errorf("connection request failed"), ErrFailedToConnect)
		}

		defer func() {
			_ = conn.Close()
			_ = revConn.Close()
		}()

		// Write initial request data
		if err := write(revConn); err != nil {
			return fmt.Errorf("failed to write initial request: %w", err)
		}

		// Create error group for managing both copy operations
		ctx, cancel := context.WithCancel(ctx)
		g, ctx := errgroup.WithContext(ctx)

		g.Go(func() error {
			<-ctx.Done()

			err1 := conn.Close()
			err2 := revConn.Close()

			return errors.Join(err1, err2)
		})

		// Copy from reverse connection to revclient connection
		g.Go(func() error {
			defer cancel()

			return pipeConn(conn, revConn)
		})

		// Copy from revclient connection to reverse connection
		g.Go(func() error {
			defer cancel()

			return pipeConn(revConn, conn)
		})

		err := g.Wait()

		return err
	}
}

// pipeConn copies data between src and dst connections bidirectionally until EOF or an error occurs.
// It returns nil if the connection is closed gracefully with EOF or net.ErrClosed.
// Returns error if any other issue occurs during data transfer.
func pipeConn(src, dst net.Conn) error {
	_, err := io.Copy(src, dst)

	switch {
	case errors.Is(err, io.EOF), errors.Is(err, net.ErrClosed):
		return nil
	case err != nil:
		return fmt.Errorf("error copying from reverse connection: %w", err)
	}

	return nil
}
