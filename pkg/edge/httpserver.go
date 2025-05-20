package edge

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/ksysoev/make-it-public/pkg/core"
	"github.com/ksysoev/make-it-public/pkg/core/url"
	"github.com/ksysoev/make-it-public/pkg/edge/middleware"
	"github.com/pires/go-proxyproto"
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
	Listen    string               `mapstructure:"listen"`
	Public    PublicEndpointConfig `mapstructure:"public"`
	ConnLimit int                  `mapstructure:"conn_limit"`
}

type PublicEndpointConfig struct {
	Schema string `mapstructure:"schema"`
	Domain string `mapstructure:"domain"`
	Port   int    `mapstructure:"port"`
}

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

	ln, err := net.Listen("tcp", s.config.Listen)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.config.Listen, err)
	}

	proxyListener := &proxyproto.Listener{
		Listener:          ln,
		ReadHeaderTimeout: 5 * time.Second,
	}

	server := &http.Server{
		Addr:              s.config.Listen,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
	}

	go func() {
		<-ctx.Done()

		_ = server.Close()
		_ = proxyListener.Close()
	}()

	if err := server.Serve(proxyListener); err != http.ErrServerClosed {
		return err
	}

	return nil
}

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
		resp := &http.Response{
			Status:     "502 Bad Gateway",
			StatusCode: http.StatusBadGateway,
			Proto:      r.Proto,
			ProtoMajor: r.ProtoMajor,
			ProtoMinor: r.ProtoMinor,
		}

		_ = resp.Write(clientConn)
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		slog.DebugContext(ctx, "connection timed out", slog.String("host", r.Host))
		return
	case err != nil:
		slog.ErrorContext(ctx, "failed to handle connection", slog.Any("error", err))
		http.Error(w, "Failed to handle connection: "+err.Error(), http.StatusInternalServerError)
	}
}
