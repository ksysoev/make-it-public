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
}

type ClientServer struct {
	token *token.Token
	cfg   Config
	wg    sync.WaitGroup
}

type Conn interface {
	net.Conn
	CloseWrite() error
}

func NewClientServer(cfg Config, tkn *token.Token) *ClientServer {
	return &ClientServer{
		cfg:   cfg,
		token: tkn,
	}
}

func (s *ClientServer) Run(ctx context.Context) error {
	opts := []revdial.ListenerOption{}

	authOpt, err := revdial.WithUserPass(s.token.ID, s.token.Secret)
	if err != nil {
		return fmt.Errorf("failed to create auth option: %w", err)
	}

	opts = append(opts, authOpt)

	onConnect, err := revdial.WithEventHandler("urlToConnectUpdated", func(event revdial.Event) {
		var url string
		if err := event.ParsePayload(&url); err != nil {
			slog.ErrorContext(ctx, "failed to parse payload for event urlToConnectUpdated", "error", err)
		}

		slog.InfoContext(ctx, "Client url to connect", "url", url)
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

	listener, err := revdial.Listen(ctx, s.cfg.ServerAddr, opts...)
	if err != nil {
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

	slog.InfoContext(ctx, "new incoming connection", "clientIP", connMeta.IP)
	defer slog.InfoContext(ctx, "closing connection", "clientIP", connMeta.IP)

	d := net.Dialer{
		Timeout: 5 * time.Second,
	}

	destConn, err := d.DialContext(ctx, "tcp", s.cfg.DestAddr)
	if err != nil {
		slog.ErrorContext(ctx, "failed to dial", "err", err)
		return
	}

	conn1, ok := conn.(Conn)
	conn2, ok2 := destConn.(Conn)

	if !ok || !ok2 {
		slog.ErrorContext(ctx, "failed to cast connections to custom Conn interface")
		return
	}

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(pipeConn(ctx, conn1, conn2))
	eg.Go(pipeConn(ctx, conn2, conn1))

	go func() {
		<-ctx.Done()

		_ = conn2.Close()
		_ = conn1.Close()
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
