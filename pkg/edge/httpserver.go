package edge

import (
	"context"
	"net"
	"net/http"
	"time"
)

type ConnService interface {
	HandleHTTPConnection(ctx context.Context, userID string, conn net.Conn, write func(net.Conn) error) error
}

type HTTPServer struct {
	connService ConnService
	listen      string
}

func New(listen string, connService ConnService) *HTTPServer {
	return &HTTPServer{
		listen:      listen,
		connService: connService,
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
	// for now we just get the user id from the header, but if future we will take it from subdomain
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "missing user id", http.StatusBadRequest)
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

	err = s.connService.HandleHTTPConnection(r.Context(), userID, clientConn, func(conn net.Conn) error {
		return r.Write(conn)
	})

	if err != nil {
		http.Error(w, "Failed to handle connection: "+err.Error(), http.StatusInternalServerError)
	}

	return
}
