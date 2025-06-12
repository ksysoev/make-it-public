package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"syscall"
	"time"

	"github.com/ksysoev/make-it-public/pkg/core/conn"
	"github.com/ksysoev/make-it-public/pkg/core/conn/meta"
	"github.com/ksysoev/revdial/proto"
	"golang.org/x/sync/errgroup"
)

const (
	connectionTimeout = 5 * time.Second
)

var (
	ErrFailedToConnect = errors.New("failed to connect")
	ErrKeyIDNotFound   = errors.New("keyID not found")
)

type Conn interface {
	Read(p []byte) (n int, err error)
	Write(p []byte) (n int, err error)
	Close() error
	CloseWrite() error
}

func (s *Service) HandleReverseConn(ctx context.Context, revConn net.Conn) error {
	ctx, cancelTimeout := timeoutContext(ctx, connectionTimeout)

	slog.DebugContext(ctx, "new connection", slog.Any("remote", revConn.RemoteAddr()))
	defer slog.DebugContext(ctx, "closing connection", slog.Any("remote", revConn.RemoteAddr()))

	var connKeyID string

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

	err := servConn.Process()

	cancelTimeout()

	if err != nil {
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
	case errors.Is(err, ErrKeyIDNotFound):
		ok, err := s.auth.IsKeyExists(ctx, keyID)
		if err != nil {
			return fmt.Errorf("failed to check key existence: %w", err)
		}

		if !ok {
			return fmt.Errorf("keyID %s not found: %w", keyID, ErrKeyIDNotFound)
		}

		return fmt.Errorf("no connections available for keyID %s: %w", keyID, ErrFailedToConnect)
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

	destConn, ok := revConn.(Conn)
	if !ok {
		return fmt.Errorf("failed to cast reverse connection to custom Conn interface %T", revConn)
	}

	eg.Go(pipeToDest(connNopCloser, destConn))
	eg.Go(pipeToSource(destConn, connNopCloser))

	eg.Go(func() error {
		select {
		case <-ctx.Done():
		case <-req.ParentContext().Done(): // Pare
		}

		return revConn.Close()
	})

	if err := eg.Wait(); !errors.Is(err, net.ErrClosed) {
		return err
	}

	return nil
}

func pipeToDest(src io.Reader, dst Conn) func() error {
	return func() error {
		_, err := io.Copy(dst, src)

		slog.Info("Copying data from source to destination done")

		switch {
		case errors.Is(err, net.ErrClosed), errors.Is(err, syscall.ECONNRESET):
			return io.EOF
		case err != nil:
			return fmt.Errorf("error copying from reverse connection: %w", err)
		}

		return dst.CloseWrite()
	}
}

func pipeToSource(src Conn, dst io.Writer) func() error {
	return func() error {
		_, err := io.Copy(dst, src)

		slog.Info("Copying data from destination to source done")

		switch {
		case errors.Is(err, net.ErrClosed), errors.Is(err, syscall.ECONNRESET):
			return net.ErrClosed
		case err != nil:
			return fmt.Errorf("error copying to reverse connection: %w", err)
		}

		return src.Close()
	}
}

// timeoutContext creates a new context with a specified timeout duration.
// It cancels the context either when the timeout elapses or the parent context is canceled.
// Accepts ctx as the parent context and timeout specifying the duration before cancellation.
// Returns the new context and a cancel function to release resources. The cancel function should always be called to avoid leaks.
func timeoutContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}

	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	wg := sync.WaitGroup{}

	wg.Add(1)

	go func() {
		defer wg.Done()

		select {
		case <-ctx.Done():
			return
		case <-time.After(timeout):
			cancel()
		case <-done:
			return
		}
	}()

	return ctx, func() {
		close(done)
		wg.Wait()
	}
}
