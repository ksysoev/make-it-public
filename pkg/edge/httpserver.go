package edge

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ConnService interface {
	HandleHTTPConnection(ctx context.Context, userID string, conn net.Conn, write func(net.Conn) error) error
}

type HTTPServer struct {
	connService ConnService
	config      Config
}

type Config struct {
	Listen string `mapstructure:"listen"`
	Domain string `mapstructure:"domain"`
}

func New(cfg Config, connService ConnService) *HTTPServer {
	return &HTTPServer{
		config:      cfg,
		connService: connService,
	}
}

func (s *HTTPServer) Run(ctx context.Context) error {
	server := &http.Server{
		Addr:              s.config.Listen,
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
	reqID := uuid.New().String()
	ctx := context.WithValue(r.Context(), "req_id", reqID)

	if !strings.HasSuffix(r.Host, s.config.Domain) {
		http.Error(w, "request is not sent to the defined domain", http.StatusBadRequest)
		return
	}

	userID := s.getUserIDFromHeader(r)
	if userID == "" {
		http.Error(w, "invalid or missing subdomain", http.StatusBadRequest)
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		slog.ErrorContext(ctx, "webserver doesn't support hijacking", slog.String("host", r.Host))
		http.Error(w, "webserver doesn't support hijacking", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hj.Hijack()
	if err != nil {
		slog.ErrorContext(ctx, "failed to hijack connection", slog.Any("error", err))
		http.Error(w, "Failed to hijack connection: "+err.Error(), http.StatusInternalServerError)
		return
	}

	defer func() { _ = clientConn.Close() }()

	err = s.connService.HandleHTTPConnection(ctx, userID, clientConn, func(conn net.Conn) error {
		return r.Write(conn)
	})

	if err != nil {
		slog.ErrorContext(ctx, "failed to handle connection", slog.Any("error", err))
		http.Error(w, "Failed to handle connection: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

// getUserIDFromHeader extracts the subdomain from the host in the HTTP request.
// It assumes the host follows the subdomain.domain.tld format.
// Returns the subdomain as a string or an empty string if no subdomain exists.
func (s *HTTPServer) getUserIDFromHeader(r *http.Request) string {
	host := r.Host

	if host != "" {
		parts := strings.Split(host, ".")
		if len(parts) > 2 {
			// Extract subdomain (assuming subdomain.domain.tld format)
			return parts[0]
		}
	}

	return ""
}
