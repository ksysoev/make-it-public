package revproxy

import (
	"context"
	"fmt"
	"net"
	"sync"
)

type ConnService interface {
	HandleReverseConn(ctx context.Context, conn net.Conn) error
}

type RevServer struct {
	connService ConnService
	listen      string
}

func New(listen string, connService ConnService) *RevServer {
	return &RevServer{
		listen:      listen,
		connService: connService,
	}
}

func (r *RevServer) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	l, err := net.Listen("tcp", r.listen)
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

			_ = r.connService.HandleReverseConn(ctx, conn)
		}()
	}
}
