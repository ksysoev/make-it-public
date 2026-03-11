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

const (
	reconnectBackoffInitial = 1 * time.Second
	reconnectBackoffMax     = 30 * time.Second
	reconnectBackoffFactor  = 2

	// defaultKeepAliveInterval is the TCP keepalive period set on the control connection.
	// A 30s interval ensures silent TCP drops (e.g. firewall idle timeouts) are detected
	// well within typical NAT/firewall session expiry windows (often 5–30 minutes).
	defaultKeepAliveInterval = 30 * time.Second
)

// Config holds the configuration for the ClientServer.
type Config struct {
	ServerAddr string
	DestAddr   string
	NoTLS      bool
	Insecure   bool
	EnableV2   bool
}

// listenFunc is the signature for creating a reverse-dial listener.
// It is a package-level type so tests can substitute a fake implementation.
type listenFunc func(ctx context.Context, addr string, opts ...revdial.ListenerOption) (net.Listener, error)

// ClientServer manages the reverse tunnel connection to the server and
// forwards incoming connections to the configured local destination.
type ClientServer struct {
	listen         listenFunc
	onConnected    func(url string)
	onReconnected  func(url string)
	onRequest      func(clientIP string)
	token          *token.Token
	cfg            Config
	initialBackoff time.Duration
	wg             sync.WaitGroup
}

// Option is a functional option for configuring ClientServer.
type Option func(*ClientServer)

// WithOnConnected sets a callback function that is called when the client
// successfully connects to the server for the first time. The callback receives
// the public URL.
func WithOnConnected(fn func(url string)) Option {
	return func(c *ClientServer) {
		c.onConnected = fn
	}
}

// WithOnReconnected sets a callback function that is called when the client
// successfully reconnects to the server after a prior disconnection. The callback
// receives the public URL. If not set, onConnected is used as a fallback.
func WithOnReconnected(fn func(url string)) Option {
	return func(c *ClientServer) {
		c.onReconnected = fn
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

// Conn extends net.Conn with CloseWrite to allow half-close of the write side.
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
// Otherwise, it wraps it in a connWrapper whose CloseWrite is a best-effort
// implementation that delegates to Close (which, for yamux streams, results
// in a write-side half-close by sending FIN).
func wrapConn(conn net.Conn) Conn {
	if c, ok := conn.(Conn); ok {
		return c
	}

	return &connWrapper{Conn: conn}
}

// NewClientServer creates a new ClientServer with the given configuration, token, and options.
func NewClientServer(cfg Config, tkn *token.Token, opts ...Option) *ClientServer {
	cs := &ClientServer{
		cfg:            cfg,
		token:          tkn,
		initialBackoff: reconnectBackoffInitial,
		listen: func(ctx context.Context, addr string, opts ...revdial.ListenerOption) (net.Listener, error) {
			return revdial.Listen(ctx, addr, opts...)
		},
	}

	for _, opt := range opts {
		opt(cs)
	}

	return cs
}

// buildOpts constructs the revdial listener options from the current configuration.
// isReconnect indicates whether the URL event handler should fire onReconnected
// instead of onConnected.
func (s *ClientServer) buildOpts(ctx context.Context, isReconnect bool) ([]revdial.ListenerOption, error) {
	var opts []revdial.ListenerOption

	authOpt, err := revdial.WithUserPass(s.token.IDWithType(), s.token.Secret)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth option: %w", err)
	}

	opts = append(opts, authOpt)

	onConnect, err := revdial.WithEventHandler("urlToConnectUpdated", func(event revdial.Event) {
		var url string
		if err := event.ParsePayload(&url); err != nil {
			slog.ErrorContext(ctx, "failed to parse payload for event urlToConnectUpdated", "error", err)
			return
		}

		if isReconnect {
			switch {
			case s.onReconnected != nil:
				s.onReconnected(url)
			case s.onConnected != nil:
				s.onConnected(url)
			default:
				slog.InfoContext(ctx, "mit client reconnected", "url", url)
			}
		} else {
			if s.onConnected != nil {
				s.onConnected(url)
			} else {
				slog.InfoContext(ctx, "mit client is connected", "url", url)
			}
		}
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create event handler: %w", err)
	}

	opts = append(opts, onConnect)

	if !s.cfg.NoTLS {
		host, _, err := net.SplitHostPort(s.cfg.ServerAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to split host and port: %w", err)
		}

		tlsConf := revdial.WithListenerTLSConfig(&tls.Config{
			ServerName:         host,
			InsecureSkipVerify: s.cfg.Insecure, //nolint:gosec // default value is false but for testing we can skip it
			MinVersion:         tls.VersionTLS13,
		})

		opts = append(opts, tlsConf)
	}

	if s.cfg.EnableV2 {
		opts = append(opts, revdial.WithEnableV2())
	}

	opts = append(opts, revdial.WithListenerKeepAlive(defaultKeepAliveInterval))

	return opts, nil
}

// Run connects to the server and forwards incoming connections to the local destination.
// If the connection is lost after a successful connection, Run automatically reconnects
// with exponential backoff (1s up to 30s). The first connection failure is returned
// immediately without retrying. Run returns nil when ctx is cancelled.
func (s *ClientServer) Run(ctx context.Context) error {
	slog.DebugContext(ctx, "initializing revdial client",
		slog.String("server", s.cfg.ServerAddr),
		slog.Bool("v2_enabled", s.cfg.EnableV2),
		slog.Bool("no_tls", s.cfg.NoTLS),
		slog.Bool("insecure", s.cfg.Insecure))

	defer s.wg.Wait()

	backoff := s.initialBackoff
	attempt := 0

	for {
		if ctx.Err() != nil {
			return nil
		}

		opts, err := s.buildOpts(ctx, attempt > 0)
		if err != nil {
			return err
		}

		slog.DebugContext(ctx, "connecting to server",
			slog.String("server", s.cfg.ServerAddr),
			slog.Int("attempt", attempt+1))

		listener, err := s.listen(ctx, s.cfg.ServerAddr, opts...)
		if err != nil {
			if attempt == 0 {
				// If context was cancelled while the first listen was in progress,
				// treat it as a clean shutdown rather than an error.
				if ctx.Err() != nil {
					return nil
				}

				// First connection failed — report immediately, no retry.
				slog.ErrorContext(ctx, "failed to connect to server",
					slog.Any("error", err),
					slog.String("server", s.cfg.ServerAddr),
					slog.Bool("v2_enabled", s.cfg.EnableV2),
					slog.String("hint", "If connection fails, try using --disable-v2 flag for V1 fallback"))

				return err
			}

			// Subsequent reconnect attempt failed — back off and retry.
			slog.WarnContext(ctx, "reconnect attempt failed, retrying",
				slog.Any("error", err),
				slog.String("server", s.cfg.ServerAddr),
				slog.Duration("backoff", backoff))

			select {
			case <-ctx.Done():
				return nil
			case <-time.After(backoff):
			}

			backoff = min(backoff*reconnectBackoffFactor, reconnectBackoffMax)
			attempt++

			continue
		}

		// Successful connection — register a goroutine to close the listener on context
		// cancellation. The goroutine always calls listener.Close() after either branch
		// fires so the listener is never leaked when listenAndServe returns for any reason.
		stopListener := make(chan struct{})

		go func() {
			select {
			case <-ctx.Done():
			case <-stopListener:
			}

			_ = listener.Close()
		}()

		serveErr := s.listenAndServe(ctx, listener)

		// Signal the closer goroutine that the listener is already done.
		close(stopListener)

		if ctx.Err() != nil {
			// Context was cancelled — clean exit.
			return nil
		}

		// Successful connection existed — reset backoff on clean disconnect so the
		// first reconnect is always fast, rather than inheriting a prior penalty.
		// Reset before logging so the logged value matches the actual wait.
		backoff = s.initialBackoff
		attempt++

		if serveErr != nil && serveErr != revdial.ErrListenerClosed {
			// Unexpected error that is not a normal disconnect.
			slog.ErrorContext(ctx, "unexpected error from listener, will reconnect",
				slog.Any("error", serveErr),
				slog.String("server", s.cfg.ServerAddr),
				slog.Duration("backoff", backoff))
		} else {
			slog.InfoContext(ctx, "disconnected from server, reconnecting",
				slog.String("server", s.cfg.ServerAddr),
				slog.Duration("backoff", backoff))
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}

		backoff = min(backoff*reconnectBackoffFactor, reconnectBackoffMax)
	}
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

	// Ensure destConn is fully closed after piping completes
	defer func() { _ = destConn.Close() }()

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
