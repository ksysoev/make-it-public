package revproxy

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"github.com/ksysoev/revdial/proto"
)

type RevServer struct {
	listen string
}

func New(listen string, users map[string]string) *RevServer {
	return &RevServer{
		listen: listen,
	}
}

func (s *RevServer) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	l, err := net.Listen("tcp", s.listen)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	go func() {
		<-ctx.Done()

		_ = l.Close()
	}()

	wg := sync.WaitGroup{}

	for {
		conn, err := l.Accept()
		if err != nil {
			return fmt.Errorf("failed to accept connection: %w", err)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { _ = conn.Close() }()

			handleConn(ctx, conn)
		}()
	}
}

func handleConn(ctx context.Context, conn net.Conn) {
	s := proto.NewServer(conn, d.serverOpts...)

	if err := s.Process(); err != nil {
		slog.Debug("failed to process connection", slog.Any("error", err))
		return
	}

	switch s.State() {
	case proto.StateRegistered:
		d.cm.AddConnection(s)

	case proto.StateBound:
		id := s.ID()
		req := d.removeRequest(id)

		if req == nil {
			return
		}

		select {
		case req.ch <- conn:
		case <-req.ctx.Done():
			_ = s.Close()
		}

	default:
		slog.ErrorContext(ctx, "unexpected state while handling incomming connection", slog.Any("state", s.State()))
	}
}

func (s *RevServer) Stop() error {
	return nil
}

func (s *RevServer) Dial(ctx context.Context, _ string) (net.Conn, error) {
	return nil, fmt.Errorf("not implemented")
}
