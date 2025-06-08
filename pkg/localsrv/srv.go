package localsrv

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

type LocalServer struct {
	addr    string
	isReady chan struct{}
}

func New() *LocalServer {
	return &LocalServer{
		isReady: make(chan struct{}),
	}
}

func (s *LocalServer) Run(ctx context.Context) error {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		close(s.isReady)
		return fmt.Errorf("failed to start local server: %w", err)
	}

	s.addr = l.Addr().String()

	srv := http.Server{
		Handler:           s,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		if err := srv.Close(); err != nil {
			fmt.Printf("Error closing server: %v\n", err)
		}
	}()

	close(s.isReady)

	return srv.Serve(l)
}

func (s *LocalServer) Addr() string {
	<-s.isReady
	return s.addr
}

func (s *LocalServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Request: %s\n", r.URL.Path)
	fmt.Printf("Method: %s\n", r.Method)

	headers := r.Header
	for key, values := range headers {
		for _, value := range values {
			fmt.Printf("%s: %s\n", key, value)
		}
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Hello from LocalServer!"))
}
