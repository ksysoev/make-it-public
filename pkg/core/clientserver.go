package core

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/ksysoev/make-it-public/pkg/core/token"
	"github.com/ksysoev/revdial"
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

	listener, err := revdial.Listen(ctx, s.serverAddr, authOpt)
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
	defer conn.Close()

	d := net.Dialer{
		Timeout: 5 * time.Second,
	}

	destConn, err := d.DialContext(ctx, "tcp", s.destAddr)
	if err != nil {
		slog.ErrorContext(ctx, "failed to dial", "err", err)
		return
	}

	defer destConn.Close()

	done := make(chan struct{}, 2)

	go func() {
		_, _ = io.Copy(destConn, conn)

		done <- struct{}{}
	}()

	go func() {
		_, _ = io.Copy(conn, destConn)

		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}
}
