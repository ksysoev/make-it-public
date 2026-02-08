package revclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/ksysoev/make-it-public/pkg/core/conn/meta"
	"github.com/ksysoev/make-it-public/pkg/core/token"
	"github.com/ksysoev/revdial"
	"golang.org/x/sync/errgroup"
)

type Config struct {
	ServerAddr string
	DestAddr   string
	NoTLS      bool
	Insecure   bool
	EnableV2   bool
}

type ClientServer struct {
	onConnected func(url string)
	onRequest   func(clientIP string)
	token       *token.Token
	cfg         Config
	wg          sync.WaitGroup
}

// Option is a functional option for configuring ClientServer.
type Option func(*ClientServer)

// WithOnConnected sets a callback function that is called when the client
// successfully connects to the server. The callback receives the public URL.
func WithOnConnected(fn func(url string)) Option {
	return func(c *ClientServer) {
		c.onConnected = fn
	}
}

// WithOnRequest sets a callback function that is called for each incoming request.
// The callback receives the client IP address. When set, it replaces the default
// slog message for a cleaner interactive display.
func WithOnRequest(fn func(clientIP string)) Option {
	return func(c *ClientServer) {
		c.onRequest = fn
	}
}

type Conn interface {
	net.Conn
	CloseWrite() error
}

// connWrapper wraps a net.Conn that doesn't natively expose CloseWrite() (like yamux.Stream).
// CloseWrite delegates to Close(), which for yamux sends a FIN and transitions to half-closed state.
type connWrapper struct {
	net.Conn
}

func (w *connWrapper) CloseWrite() error {
	return w.Close()
}

// wrapConn wraps a net.Conn to satisfy the Conn interface.
// If the connection already implements CloseWrite(), it returns the connection as-is.
// Otherwise, it wraps it in a connWrapper that provides a no-op CloseWrite().
func wrapConn(conn net.Conn) Conn {
	if c, ok := conn.(Conn); ok {
		return c
	}

	return &connWrapper{Conn: conn}
}

func NewClientServer(cfg Config, tkn *token.Token, opts ...Option) *ClientServer {
	cs := &ClientServer{
		cfg:   cfg,
		token: tkn,
	}

	for _, opt := range opts {
		opt(cs)
	}

	return cs
}

func (s *ClientServer) Run(ctx context.Context) error {
	opts := []revdial.ListenerOption{}

	slog.DebugContext(ctx, "initializing revdial client",
		slog.String("server", s.cfg.ServerAddr),
		slog.Bool("v2_enabled", s.cfg.EnableV2),
		slog.Bool("no_tls", s.cfg.NoTLS),
		slog.Bool("insecure", s.cfg.Insecure))

	authOpt, err := revdial.WithUserPass(s.token.ID, s.token.Secret)
	if err != nil {
		return fmt.Errorf("failed to create auth option: %w", err)
	}

	opts = append(opts, authOpt)

	onConnect, err := revdial.WithEventHandler("urlToConnectUpdated", func(event revdial.Event) {
		var url string
		if err := event.ParsePayload(&url); err != nil {
			slog.ErrorContext(ctx, "failed to parse payload for event urlToConnectUpdated", "error", err)
			return
		}

		// Call the onConnected callback if set (for interactive display)
		if s.onConnected != nil {
			s.onConnected(url)
		} else {
			// Fall back to slog for non-interactive mode
			slog.InfoContext(ctx, "mit client is connected", "url", url)
		}
	})
	if err != nil {
		return fmt.Errorf("failed to create event handler: %w", err)
	}

	opts = append(opts, onConnect)

	if !s.cfg.NoTLS {
		host, _, err := net.SplitHostPort(s.cfg.ServerAddr)
		if err != nil {
			return fmt.Errorf("failed to split host and port: %w", err)
		}

		tlsConf := revdial.WithListenerTLSConfig(&tls.Config{
			ServerName:         host,
			InsecureSkipVerify: s.cfg.Insecure, //nolint:gosec // default value is false but for testing we can skip it
			MinVersion:         tls.VersionTLS13,
		})

		opts = append(opts, tlsConf)
	}

	// Enable V2 protocol if configured for improved performance with multiplexing
	if s.cfg.EnableV2 {
		slog.DebugContext(ctx, "enabling V2 protocol with yamux multiplexing")

		opts = append(opts, revdial.WithEnableV2())
	} else {
		slog.DebugContext(ctx, "V2 disabled, using V1 protocol (use without --disable-v2 to enable V2)")
	}

	slog.DebugContext(ctx, "connecting to server", slog.String("server", s.cfg.ServerAddr))

	listener, err := revdial.Listen(ctx, s.cfg.ServerAddr, opts...)
	if err != nil {
		slog.ErrorContext(ctx, "failed to connect to server",
			slog.Any("error", err),
			slog.String("server", s.cfg.ServerAddr),
			slog.Bool("v2_enabled", s.cfg.EnableV2),
			slog.String("hint", "If connection fails, try using --disable-v2 flag for V1 fallback"))

		return err
	}

	go func() {
		<-ctx.Done()

		_ = listener.Close()
	}()

	defer s.wg.Wait()

	err = s.listenAndServe(ctx, listener)
	if err != nil && err != revdial.ErrListenerClosed {
		return err
	}

	return nil
}

func (s *ClientServer) listenAndServe(ctx context.Context, listener net.Listener) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}

		s.wg.Add(1)

		go func() {
			defer s.wg.Done()

			s.handleConn(ctx, conn)
		}()
	}
}

func (s *ClientServer) handleConn(ctx context.Context, conn net.Conn) {
	defer func() { _ = conn.Close() }()

	var connMeta meta.ClientConnMeta
	if err := meta.ReadData(conn, &connMeta); err != nil {
		slog.ErrorContext(ctx, "failed to read connection metadata", "error", err)
		return
	}

	// Use callback for interactive display, otherwise use slog
	if s.onRequest != nil {
		s.onRequest(connMeta.IP)
	} else {
		slog.InfoContext(ctx, "new incoming connection", "clientIP", connMeta.IP)
	}

	defer slog.DebugContext(ctx, "closing connection", "clientIP", connMeta.IP)

	d := net.Dialer{
		Timeout: 5 * time.Second,
	}

	dConn, err := d.DialContext(ctx, "tcp", s.cfg.DestAddr)
	if err != nil {
		slog.ErrorContext(ctx, "failed to dial", "err", err)
		return
	}

	// Wrap connections to ensure they implement the Conn interface (with CloseWrite support)
	destConn := wrapConn(dConn)
	revConn := wrapConn(conn)

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(pipeConn(ctx, revConn, destConn))
	eg.Go(pipeConn(ctx, destConn, revConn))

	go func() {
		<-ctx.Done()

		_ = destConn.Close()
		_ = revConn.Close()
	}()

	if err := eg.Wait(); err != nil {
		slog.DebugContext(ctx, "error during connection data transfer", slog.Any("error", err))
	}
}

// pipeConn facilitates data transfer from the source connection to the destination connection in a single direction.
// It utilizes io.Copy for copying data and closes the writing end of the destination connection afterward.
// Accepts src as the source Conn interface and dst as the destination Conn interface, both supporting a CloseWrite method.
// Returns a function that executes the transfer process, returning an error if copying fails or if closing dst's write end fails.
func pipeConn(ctx context.Context, src, dst Conn) func() error {
	return func() error {
		n, err := io.Copy(dst, src)
		slog.DebugContext(ctx, "data copied", slog.Int64("bytes_written", n), slog.Any("error", err))

		if err != nil {
			return fmt.Errorf("error copying data: %w", err)
		}

		return dst.CloseWrite()
	}
}
