package edge

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ksysoev/make-it-public/pkg/core"
	"github.com/ksysoev/make-it-public/pkg/core/url"
	"github.com/ksysoev/make-it-public/pkg/edge/middleware"
)

type ConnService interface {
	HandleHTTPConnection(ctx context.Context, keyID string, conn net.Conn, write func(net.Conn) error, clientIP string) error
	SetEndpointGenerator(generator func(string) (string, error))
}

type HTTPServer struct {
	connService ConnService
	config      Config
}

const defaultConnLimitPerKeyID = 4

type Config struct {
	Listen     string               `mapstructure:"listen"`
	Public     PublicEndpointConfig `mapstructure:"public"`
	ConnLimit  int                  `mapstructure:"conn_limit"`
	ProxyProto bool                 `mapstructure:"proxy_proto"`
}

type PublicEndpointConfig struct {
	Schema string `mapstructure:"schema"`
	Domain string `mapstructure:"domain"`
	Port   int    `mapstructure:"port"`
}

// New initializes and returns an instance of HTTPServer configured with the provided settings and connection service.
// It validates the configuration by creating a URL endpoint generator and applies it to the connection service.
// Accepts cfg, a configuration struct defining server and public endpoint parameters, and connService,
// an interface to manage HTTP connections.
// Returns a pointer to an HTTPServer if successful or an error if the configuration or endpoint generator fails.
func New(cfg Config, connService ConnService) (*HTTPServer, error) {
	generator, err := url.NewEndpointGenerator(cfg.Public.Schema, cfg.Public.Domain, cfg.Public.Port)
	if err != nil {
		return nil, fmt.Errorf("failed to create endpoint generator: %w", err)
	}

	connService.SetEndpointGenerator(generator)

	return &HTTPServer{
		config:      cfg,
		connService: connService,
	}, nil
}

// Run starts the HTTP server and manages its lifecycle using the provided context.
// It composes middleware, sets up a TCP listener, and creates an HTTP server instance.
// Accepts ctx to control the server's lifecycle and handle graceful shutdowns.
// Returns an error if the server fails to start, listen, or encounters unexpected termination issues.
func (s *HTTPServer) Run(ctx context.Context) error {
	var mw []func(next http.Handler) http.Handler

	mw = append(mw,
		middleware.NewFishingProtection(),
		middleware.ParseKeyID(s.config.Public.Domain),
		middleware.Metrics(),
		middleware.LimitConnections(cmp.Or(s.config.ConnLimit, defaultConnLimitPerKeyID)),
		middleware.ClientIP(),
	)

	var handler http.Handler = s

	for i := len(mw) - 1; i >= 0; i-- {
		handler = mw[i](handler)
	}

	ln, err := listen(s.config.Listen, s.config.ProxyProto)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.config.Listen, err)
	}

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
	}

	go func() {
		<-ctx.Done()

		_ = server.Close()
	}()

	if err := server.Serve(ln); err != http.ErrServerClosed {
		return err
	}

	return nil
}

// ServeHTTP handles incoming HTTP requests by processing the request context and managing hijacked connections.
// It uses a hijacker to take control of the underlying connection for advanced protocol handling.
// Returns appropriate HTTP error responses for unsupported hijacking, connection issues, or context errors.
func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//nolint:staticcheck,revive // don't want to couple with cmd package for now
	ctx := context.WithValue(r.Context(), "req_id", uuid.New().String())

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

	keyID := middleware.GetKeyID(r)
	clientIP := middleware.GetClientIP(r)

	err = s.connService.HandleHTTPConnection(ctx, keyID, clientConn, func(conn net.Conn) error {
		return r.Write(conn)
	}, clientIP)

	switch {
	case errors.Is(err, core.ErrFailedToConnect):
		sendResponse(r, clientConn, http.StatusBadGateway, htmlErrorTemplate502)
	case errors.Is(err, core.ErrKeyIDNotFound):
		sendResponse(r, clientConn, http.StatusNotFound, htmlErrorTemplate404)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		slog.DebugContext(ctx, "connection timed out", slog.String("host", r.Host))
	case err != nil:
		slog.ErrorContext(ctx, "failed to handle connection", slog.Any("error", err))
	}
}

// sendResponse constructs and sends an HTTP response over a hijacked connection.
// It builds the response using the provided request protocol details, status code, and body content.
// r is the original HTTP request from which protocol details are extracted.
// conn is the hijacked network connection used to write the response.
// status specifies the HTTP status code for the response.
// body contains the response body content as a string.
// Returns nothing but logs an error if writing the response fails.
func sendResponse(r *http.Request, conn net.Conn, status int, body string) {
	resp := http.Response{
		StatusCode:    status,
		Proto:         r.Proto,
		ProtoMajor:    r.ProtoMajor,
		ProtoMinor:    r.ProtoMinor,
		ContentLength: int64(len(body)),
		Header:        http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
		Body:          io.NopCloser(strings.NewReader(body)),
	}

	if err := resp.Write(conn); err != nil {
		slog.ErrorContext(r.Context(), "failed to write response", slog.Any("error", err))
		return
	}
}
