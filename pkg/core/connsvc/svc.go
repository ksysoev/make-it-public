package connsvc

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/google/uuid"
	"github.com/ksysoev/revdial/proto"
	"golang.org/x/sync/errgroup"
)

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

	servConn := proto.NewServer(conn, proto.WithUserPassAuth(func(user, pass string) bool {
		if s.auth.Verify(user, pass) {
			connUser = user
			return true
		}

		return false
	}))

	if err := servConn.Process(); err != nil {
		slog.Info("failed to process connection", slog.Any("error", err))
		return nil
	}

	switch servConn.State() {
	case proto.StateRegistered:
		s.connmng.AddConnection(connUser, servConn)
	case proto.StateBound:
		s.connmng.ResolveRequest(servConn.ID(), conn)
	default:
		slog.ErrorContext(ctx, "unexpected state while handling incomming connection", slog.Any("state", servConn.State()))
	}

	<-ctx.Done()

	return nil
}

func (s *Service) HandleHTTPConnection(ctx context.Context, userID string, conn net.Conn, write func(net.Conn) error) error {
	ch, err := s.connmng.RequestConnection(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to request connection: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case revConn, ok := <-ch:
		if !ok {
			return fmt.Errorf("connection request failed")
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
		g, gctx := errgroup.WithContext(ctx)

		// Copy from reverse connection to client connection
		g.Go(func() error {
			_, err := io.Copy(conn, revConn)
			if err != nil && err != io.EOF {
				return fmt.Errorf("error copying from reverse connection: %w", err)
			}
			return nil
		})

		// Copy from client connection to reverse connection
		g.Go(func() error {
			_, err := io.Copy(revConn, conn)
			if err != nil && err != io.EOF {
				return fmt.Errorf("error copying to reverse connection: %w", err)
			}
			return nil
		})

		// Wait for both copy operations to complete or context to be cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-gctx.Done():
			return gctx.Err()
		case err := <-func() chan error {
			ch := make(chan error, 1)
			go func() {
				ch <- g.Wait()
			}()
			return ch
		}():
			return err
		}
	}
}
