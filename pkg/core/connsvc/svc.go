package connsvc

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/google/uuid"
	"github.com/ksysoev/revdial/proto"
)

type ConnManager interface {
	RequestConnection(ctx context.Context, userID string) (chan net.Conn, error)
	AddConnection(user string, conn *proto.Server)
	ResolveRequest(id uuid.UUID, conn net.Conn)
}

type Service struct {
	connmng  ConnManager
	authRepo map[string]string
}

func New(connmng ConnManager) *Service {
	return &Service{
		connmng: connmng,
	}
}

func (s *Service) HandleReverseConn(ctx context.Context, conn net.Conn) error {
	var connUser string

	servConn := proto.NewServer(conn, proto.WithUserPassAuth(func(user, pass string) bool {
		if p, ok := s.authRepo[user]; ok {
			connUser = user
			return p == pass
		}

		return false
	}))

	if err := servConn.Process(); err != nil {
		slog.Debug("failed to process connection", slog.Any("error", err))
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

		go func() {
			_ = write(revConn)
		}()

		_, _ = io.Copy(conn, revConn)
	}

	return nil
}
