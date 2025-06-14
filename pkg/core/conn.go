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
	ErrConnClosed      = errors.New("connection closed")
)

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
		notifier, err := conn.NewCloseNotifier(revConn)
		if err != nil {
			return fmt.Errorf("failed to create close notifier: %w", err)
		}

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
	respBytesWritten := int64(0)

	eg.Go(pipeToDest(connNopCloser, revConn))
	eg.Go(pipeToSource(revConn, connNopCloser, &respBytesWritten))

	guard := closeOnContextDone(ctx, req.ParentContext(), revConn)
	defer guard.Wait()

	err = eg.Wait()

	if respBytesWritten <= 0 {
		return fmt.Errorf("no data written to reverse connection: %w", ErrFailedToConnect)
	}

	if err != nil && !errors.Is(err, ErrConnClosed) {
		return fmt.Errorf("failed to copy data: %w", err)
	}

	return nil
}

// pipeToDest copies data from the source Reader to the destination Conn in a streaming manner.
// It manages specific error conditions such as closed or reset connections.
// Returns a function that executes the copy process, returning ErrConnClosed for io.ErrClosedPipe or connection reset errors.
// Also returns a wrapped error for other errors encountered during the copy process, or nil if the operation completes successfully.
func pipeToDest(src io.Reader, dst conn.WithWriteCloser) func() error {
	return func() error {
		_, err := io.Copy(dst, src)

		switch {
		case errors.Is(err, net.ErrClosed), errors.Is(err, syscall.ECONNRESET):
			return ErrConnClosed
		case err != nil:
			return fmt.Errorf("error copying from reverse connection: %w", err)
		}

		if err := dst.CloseWrite(); err != nil && !errors.Is(err, net.ErrClosed) {
			return fmt.Errorf("failed to close write end of reverse connection: %w", err)
		}

		return nil
	}
}

// pipeToSource copies data from the source connection to the destination writer in a streaming manner.
// It logs the completion of the copy operation and handles specific error conditions.
// Returns a function that executes the copy process, returning ErrConnClosed if the source connection is closed or reset,
// or a wrapped error if other errors occur during the copying process.
func pipeToSource(src conn.WithWriteCloser, dst io.Writer, written *int64) func() error {
	return func() error {
		var err error

		*written, err = io.Copy(dst, src)

		switch {
		case errors.Is(err, net.ErrClosed), errors.Is(err, syscall.ECONNRESET):
			return ErrConnClosed
		case err != nil:
			return fmt.Errorf("error copying to reverse connection: %w", err)
		}

		return ErrConnClosed
	}
}

// closeOnContextDone closes the provided connection when either of the given contexts is done.
// It initiates a goroutine that waits for completion signals from reqCtx or parentCtx.
// Accepts reqCtx as the request-level context, parentCtx as the parent context, and c as the connection to close.
// Returns a *sync.WaitGroup which can be used to wait until the closing operation is complete.
func closeOnContextDone(reqCtx, parentCtx context.Context, c conn.WithWriteCloser) *sync.WaitGroup {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()

		select {
		case <-reqCtx.Done():
		case <-parentCtx.Done(): // Pare
		}

		if err := c.Close(); err != nil {
			slog.DebugContext(reqCtx, "failed to close connection", slog.Any("error", err))
		}
	}()

	return wg
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
