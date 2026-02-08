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

	"github.com/google/uuid"
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

	// Use NewServerV2 to support both V1 and V2 protocols
	// V2 provides yamux multiplexing for better performance
	baseOpts := []proto.ServerOption{
		proto.WithUserPassAuth(func(keyID, secret string) bool {
			valid, err := s.auth.Verify(ctx, keyID, secret)
			if err == nil && valid {
				connKeyID = keyID
				return true
			} else if err != nil {
				slog.ErrorContext(ctx, "failed to verify user", slog.Any("error", err))
			}

			return false
		}),
	}

	servConn := proto.NewServerV2(revConn, baseOpts)

	err := servConn.Process()

	cancelTimeout()

	if err != nil {
		slog.DebugContext(ctx, "failed to process connection", slog.Any("error", err))
		return nil
	}

	// Log protocol version for debugging
	protocolVersion := "V1"
	if servConn.IsV2() {
		protocolVersion = "V2"
	}

	slog.DebugContext(ctx, "connection processed",
		slog.String("protocol", protocolVersion),
		slog.String("keyID", connKeyID))

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

		protocolVersion := "V1"
		if servConn.IsV2() {
			protocolVersion = "V2 (yamux multiplexed)"
		}

		slog.InfoContext(ctx, "control conn established",
			slog.String("keyID", connKeyID),
			slog.String("protocol", protocolVersion))

		// For V2 connections, start accepting yamux streams in the background.
		// The client opens new streams (instead of new TCP connections) for each data connection.
		if servConn.IsV2() {
			go s.acceptV2Streams(srvConn.Context(), servConn, connKeyID)
		}

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
		slog.DebugContext(ctx, "bound connection closed", slog.String("keyID", connKeyID))

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

	eg, ctx := errgroup.WithContext(ctx)
	connNopCloser := conn.NewContextConnNopCloser(ctx, cliConn)
	respBytesWritten := int64(0)

	eg.Go(pipeToDest(ctx, connNopCloser, revConn))
	eg.Go(pipeToSource(ctx, revConn, connNopCloser, &respBytesWritten))

	guard := closeOnContextDone(ctx, req.ParentContext(), revConn)
	defer guard.Wait()

	err = eg.Wait()

	if respBytesWritten <= 0 {
		slog.DebugContext(ctx, "no data written to reverse connection", slog.Any("error", err))
		return fmt.Errorf("no data written to reverse connection: %w", ErrFailedToConnect)
	}

	if err != nil && !errors.Is(err, ErrConnClosed) {
		slog.DebugContext(ctx, "failed to copy data", slog.Any("error", err))
		return fmt.Errorf("failed to copy data: %w", err)
	}

	return nil
}

// acceptV2Streams accepts yamux streams from a V2 connection and resolves pending connection requests.
// Each stream carries a bind command with a UUID that maps to a pending request from HandleHTTPConnection.
func (s *Service) acceptV2Streams(ctx context.Context, servConn *proto.ServerV2, keyID string) {
	// Start a goroutine to close the session when context is canceled.
	// This ensures AcceptStream() is unblocked, allowing for deterministic shutdown.
	go func() {
		<-ctx.Done()

		if session := servConn.Session(); session != nil {
			_ = session.Close()
		}
	}()

	for {
		stream, err := servConn.AcceptStream()
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			slog.ErrorContext(ctx, "failed to accept V2 stream",
				slog.Any("error", err),
				slog.String("keyID", keyID))

			return
		}

		go s.handleV2Stream(ctx, stream, keyID)
	}
}

// handleV2Stream reads a bind command from a yamux stream and resolves the corresponding connection request.
// This mirrors the logic in revdial/dialer.go:handleV2Stream for the server-side stream handling.
func (s *Service) handleV2Stream(ctx context.Context, stream net.Conn, keyID string) {
	// Set a deadline for the initial bind handshake to prevent goroutine leaks
	// from clients that open streams but never send data.
	const handshakeTimeout = 10 * time.Second

	if err := stream.SetReadDeadline(time.Now().Add(handshakeTimeout)); err != nil {
		slog.ErrorContext(ctx, "failed to set read deadline on V2 stream",
			slog.Any("error", err),
			slog.String("keyID", keyID))

		_ = stream.Close()

		return
	}

	// Read version and command (2 bytes)
	buf := make([]byte, 2)
	if _, err := io.ReadFull(stream, buf); err != nil {
		slog.ErrorContext(ctx, "failed to read command from V2 stream",
			slog.Any("error", err),
			slog.String("keyID", keyID))

		_ = stream.Close()

		return
	}

	if buf[0] != proto.VersionV1() {
		slog.ErrorContext(ctx, "unexpected version in V2 stream",
			slog.Int("version", int(buf[0])),
			slog.String("keyID", keyID))

		_ = stream.Close()

		return
	}

	if buf[1] != proto.CmdBind() {
		slog.ErrorContext(ctx, "unexpected command in V2 stream",
			slog.Int("command", int(buf[1])),
			slog.String("keyID", keyID))

		_ = stream.Close()

		return
	}

	// Read UUID (16 bytes)
	uuidBuf := make([]byte, 16)
	if _, err := io.ReadFull(stream, uuidBuf); err != nil {
		slog.ErrorContext(ctx, "failed to read UUID from V2 stream",
			slog.Any("error", err),
			slog.String("keyID", keyID))

		_ = stream.Close()

		return
	}

	// Clear the read deadline now that handshake is complete
	if err := stream.SetReadDeadline(time.Time{}); err != nil {
		slog.ErrorContext(ctx, "failed to clear read deadline on V2 stream",
			slog.Any("error", err),
			slog.String("keyID", keyID))

		_ = stream.Close()

		return
	}

	id, err := uuid.FromBytes(uuidBuf)
	if err != nil {
		slog.ErrorContext(ctx, "failed to parse UUID from V2 stream",
			slog.Any("error", err),
			slog.String("keyID", keyID))

		_ = stream.Close()

		return
	}

	// Send success response
	if _, err := stream.Write([]byte{proto.VersionV1(), proto.ResSuccess()}); err != nil {
		slog.ErrorContext(ctx, "failed to write bind response on V2 stream",
			slog.Any("error", err),
			slog.String("keyID", keyID))

		_ = stream.Close()

		return
	}

	slog.DebugContext(ctx, "V2 stream bound",
		slog.String("keyID", keyID),
		slog.String("connID", id.String()))

	// Wrap the yamux stream to implement WithWriteCloser (CloseWrite)
	wrappedStream := &yamuxStreamWrapper{Conn: stream}

	notifier, err := conn.NewCloseNotifier(wrappedStream)
	if err != nil {
		slog.ErrorContext(ctx, "failed to create close notifier for V2 stream",
			slog.Any("error", err),
			slog.String("keyID", keyID))

		_ = stream.Close()

		return
	}

	s.connmng.ResolveRequest(id, notifier)

	slog.InfoContext(ctx, "V2 rev conn established", slog.String("keyID", keyID))

	notifier.WaitClose(ctx)
	slog.DebugContext(ctx, "V2 bound connection closed", slog.String("keyID", keyID))
}

// yamuxStreamWrapper wraps a yamux stream (net.Conn) to implement the WithWriteCloser interface.
// Yamux streams don't natively support half-close (CloseWrite), so CloseWrite is a no-op.
// The stream will be fully closed when Close() is called.
type yamuxStreamWrapper struct {
	net.Conn
}

// CloseWrite implements the conn.WithWriteCloser interface for yamux streams.
// Yamux doesn't support half-close, so this is a no-op.
// The full stream close happens via Close().
func (w *yamuxStreamWrapper) CloseWrite() error {
	return nil
}

// pipeToDest copies data from the source Reader to the destination Conn in a streaming manner.
// It manages specific error conditions such as closed or reset connections.
// Returns a function that executes the copy process, returning ErrConnClosed for io.ErrClosedPipe or connection reset errors.
// Also returns a wrapped error for other errors encountered during the copy process, or nil if the operation completes successfully.
func pipeToDest(ctx context.Context, src io.Reader, dst conn.WithWriteCloser) func() error {
	return func() error {
		n, err := io.Copy(dst, src)
		slog.DebugContext(ctx, "data copied to reverse connection", slog.Any("error", err), slog.Int64("bytes_written", n))

		switch {
		case errors.Is(err, net.ErrClosed), errors.Is(err, syscall.ECONNRESET):
			return ErrConnClosed
		case err != nil:
			return fmt.Errorf("error copying from reverse connection: %w", err)
		}

		if err := dst.CloseWrite(); err != nil && !errors.Is(err, net.ErrClosed) {
			slog.DebugContext(ctx, "failed to close write end of reverse connection", slog.Any("error", err))
			return fmt.Errorf("failed to close write end of reverse connection: %w", err)
		}

		return nil
	}
}

// pipeToSource copies data from the source connection to the destination writer in a streaming manner.
// It logs the completion of the copy operation and handles specific error conditions.
// Returns a function that executes the copy process, returning ErrConnClosed if the source connection is closed or reset,
// or a wrapped error if other errors occur during the copying process.
func pipeToSource(ctx context.Context, src conn.WithWriteCloser, dst io.Writer, written *int64) func() error {
	return func() error {
		var err error

		*written, err = io.Copy(dst, src)
		slog.DebugContext(ctx, "data copied from reverse connection", slog.Int64("bytes_written", *written), slog.Any("error", err))

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
			slog.DebugContext(reqCtx, "closing connection, request context done", slog.Any("error", reqCtx.Err()))
		case <-parentCtx.Done():
			slog.DebugContext(reqCtx, "closing connection, parent context done", slog.Any("error", parentCtx.Err()))
		}

		slog.DebugContext(reqCtx, "closing connection, context done")

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
