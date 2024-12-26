package core

import (
	"context"
	"net"
	"sync"

	"github.com/ksysoev/revdial"
)

type RevServer struct {
	listen string
	mu     sync.Mutex
	dialer *revdial.Dialer
}

func NewRevServer(listen string) *RevServer {
	return &RevServer{
		dialer: revdial.NewDialer(listen),
	}
}

func (s *RevServer) Start(ctx context.Context) error {
	return s.dialer.Start(ctx)
}

func (s *RevServer) Stop(ctx context.Context) error {
	return s.dialer.Stop()
}

func (s *RevServer) Dial(ctx context.Context, _ string) (net.Conn, error) {
	return s.dialer.DialContext(ctx)
}
