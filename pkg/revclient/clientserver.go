package revclient

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

	"github.com/ksysoev/make-it-public/pkg/core/token"
	"github.com/ksysoev/revdial"
	"golang.org/x/sync/errgroup"
)

type ClientServer struct {
	token      *token.Token
	serverAddr string
	destAddr   string
	wg         sync.WaitGroup
}

func NewClientServer(serverAddr, destAddr string, tkn *token.Token) *ClientServer {
	return &ClientServer{
		serverAddr: serverAddr,
		destAddr:   destAddr,
		token:      tkn,
	}
}

func (s *ClientServer) Run(ctx context.Context) error {
	authOpt, err := revdial.WithUserPass(s.token.ID, s.token.Secret)
	if err != nil {
		return fmt.Errorf("failed to create auth option: %w", err)
	}

	onConnect, err := revdial.WithEventHandler("urlToConnectUpdated", func(event revdial.Event) {
		var url string
		if err := event.ParsePayload(&url); err != nil {
			slog.ErrorContext(ctx, "failed to parse payload for event urlToConnectUpdated", "error", err)
		}

		slog.InfoContext(ctx, "Client url to connect", "url", url)
	})

	listener, err := revdial.Listen(ctx, s.serverAddr, authOpt, onConnect)
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

	d := net.Dialer{
		Timeout: 5 * time.Second,
	}

	destConn, err := d.DialContext(ctx, "tcp", s.destAddr)
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
