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

	"github.com/ksysoev/make-it-public/pkg/core/conn"
	"github.com/ksysoev/make-it-public/pkg/core/conn/meta"
	"github.com/ksysoev/revdial/proto"
	"golang.org/x/sync/errgroup"
)

var (
	ErrFailedToConnect = errors.New("failed to connect")
	ErrKeyIDNotFound   = errors.New("keyID not found")
)

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
		slog.DebugContext(ctx, "failed to process connection", slog.Any("error", err))
		return nil
	}

	switch servConn.State() {
	case proto.StateRegistered:
		srvConn := conn.NewServerConn(ctx, servConn)

		url, err := s.endpointGenerator(connKeyID)
		if err != nil {
			return fmt.Errorf("failed to generate endpoint: %w", err)
		}

		if err := srvConn.SendURLToConnectUpdatedEvent(url); err != nil {
			return fmt.Errorf("failed to send url to connect updated event: %w", err)
		}

		s.connmng.AddConnection(connKeyID, srvConn)

		defer s.connmng.RemoveConnection(connKeyID, srvConn.ID())

		slog.InfoContext(ctx, "control conn established", slog.String("keyID", connKeyID))

		for {
			select {
			case <-srvConn.Context().Done():
				return nil
			case <-time.After(200 * time.Millisecond):
			}

			err := srvConn.Ping()
			if err != nil {
				slog.DebugContext(ctx, "ping failed", slog.Any("error", err))
				return nil
			}
		}
	case proto.StateBound:
		notifier := conn.NewCloseNotifier(revConn)

		s.connmng.ResolveRequest(servConn.ID(), notifier)
		slog.InfoContext(ctx, "rev conn established", slog.String("keyID", connKeyID))

		notifier.WaitClose(ctx)

		return nil
	default:
		return fmt.Errorf("unexpected state while handling incomming connection: %d", servConn.State())
	}
}

func (s *Service) HandleHTTPConnection(ctx context.Context, keyID string, cliConn net.Conn, write func(net.Conn) error, clientIP string) error {
	slog.DebugContext(ctx, "new HTTP connection", slog.Any("remote", cliConn.RemoteAddr()))
	defer slog.DebugContext(ctx, "closing HTTP connection", slog.Any("remote", cliConn.RemoteAddr()))

	req, err := s.connmng.RequestConnection(ctx, keyID)

	switch {
	case err != nil && errors.Is(err, ErrKeyIDNotFound):
		ok, err := s.auth.IsKeyExists(ctx, keyID)
		if err != nil {
			return fmt.Errorf("failed to check key existence: %w", err)
		}

		if !ok {
			return ErrKeyIDNotFound
		}
	case err != nil:
		return fmt.Errorf("failed to request connection: %w", ErrFailedToConnect)
	}

	revConn, err := req.WaitConn(ctx)
	if err != nil {
		s.connmng.CancelRequest(req.ID())
		return fmt.Errorf("connection request failed: %w", ErrFailedToConnect)
	}

	slog.DebugContext(ctx, "connection received", slog.Any("remote", cliConn.RemoteAddr()))

	if err := meta.WriteData(revConn, &meta.ClientConnMeta{IP: clientIP}); err != nil {
		slog.DebugContext(ctx, "failed to write client connection meta", slog.Any("error", err))

		return fmt.Errorf("failed to write client connection meta: %w", ErrFailedToConnect)
	}

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
