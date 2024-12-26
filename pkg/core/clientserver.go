package core

import (
	"context"
	"net/http"

	"github.com/ksysoev/revdial"
)

type ClientServer struct {
	remote string
}

func NewClientServer(remote string) *ClientServer {
	return &ClientServer{
		remote: remote,
	}
}

func (s *ClientServer) Run(ctx context.Context) error {
	listener, err := revdial.Listen(ctx, s.remote)
	if err != nil {
		return err
	}

	serve := http.Server{
		Handler: s,
	}

	go func() {
		<-ctx.Done()
		_ = serve.Close()
	}()

	return serve.Serve(listener)
}

func (s *ClientServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hello, World!"))
}
