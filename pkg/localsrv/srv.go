package localsrv

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

type LocalServer struct {
	url string
}

func New() *LocalServer {
	return &LocalServer{}
}

func (s *LocalServer) Run(ctx context.Context) error {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return fmt.Errorf("failed to start local server: %w", err)
	}

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

	return srv.Serve(l)
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
