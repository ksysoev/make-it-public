package core

import (
	"io"
	"net/http"
)

type HTTPServer struct {
	listen     string
	revDialler *RevServer
}

func NewHTTPServer(listen string, revDialler *RevServer) *HTTPServer {
	return &HTTPServer{
		listen:     listen,
		revDialler: revDialler,
	}
}

func (s *HTTPServer) Run() error {
	server := &http.Server{
		Addr:    s.listen,
		Handler: s,
	}

	return server.ListenAndServe()
}

func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := s.revDialler.Dial(r.Context(), "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

	return
}
