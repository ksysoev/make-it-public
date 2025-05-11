package revclient

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"syscall"
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
			InsecureSkipVerify: s.cfg.Insecure,
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

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(pipeConn(conn, destConn))
	eg.Go(pipeConn(destConn, conn))
	eg.Go(func() error {
		<-ctx.Done()

		return errors.Join(conn.Close(), destConn.Close())
	})

	_ = eg.Wait()
}

// pipeConn transfers data between two network connections until an error or EOF occurs.
// It propagates specific network-related errors (like connection closures or resets) as io.EOF.
// Returns an error if an unexpected I/O error occurs during data transfer.
func pipeConn(src, dst net.Conn) func() error {
	return func() error {
		_, err := io.Copy(src, dst)

		switch {
		case errors.Is(err, net.ErrClosed), errors.Is(err, syscall.ECONNRESET):
			return io.EOF
		case err != nil:
			return fmt.Errorf("error copying from reverse connection: %w", err)
		}

		return io.EOF
	}
}
