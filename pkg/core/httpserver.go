package core

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/ksysoev/make-it-public/pkg/revproxy"
)

type HTTPServer struct {
	revDialler *revproxy.RevServer
	listen     string
}

func NewHTTPServer(listen string, revDialler *revproxy.RevServer) *HTTPServer {
	return &HTTPServer{
		listen:     listen,
		revDialler: revDialler,
	}
}

func (s *HTTPServer) Run(ctx context.Context) error {
	server := &http.Server{
		Addr:              s.listen,
		Handler:           s,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
	}

	go func() {
		<-ctx.Done()

		_ = server.Close()
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}

	return nil
}

func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := s.revDialler.Dial(r.Context(), "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hj.Hijack()
	if err != nil {
		http.Error(w, "Failed to hijack connection: "+err.Error(), http.StatusInternalServerError)
		return
	}

	defer func() { _ = clientConn.Close() }()

	// Proxy the client request to the reverse-dialed server connection.
	go func() {
		_ = r.Write(conn) // Write the HTTP request to the reverse-dialed server.
	}()

	// Proxy the server's response back to the client.
	_, _ = io.Copy(clientConn, conn)
}
