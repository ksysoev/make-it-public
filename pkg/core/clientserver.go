package core

import (
	"context"
	"net/http"
	"time"

	"github.com/ksysoev/revdial"
)

type ClientServer struct {
	serverAddr string
	destAddr   string
}

func NewClientServer(serverAddr string, destAddr string) *ClientServer {
	return &ClientServer{
		serverAddr: serverAddr,
		destAddr:   destAddr,
	}
}

func (s *ClientServer) Run(ctx context.Context) error {
	listener, err := revdial.Listen(ctx, s.serverAddr)
	if err != nil {
		return err
	}

	serve := http.Server{
		Handler:           s,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
	}

	go func() {
		<-ctx.Done()

		_ = serve.Close()
	}()

	return serve.Serve(listener)
}

func (s *ClientServer) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write([]byte("Hello, World!"))
}
