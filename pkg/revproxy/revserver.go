package revproxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"github.com/google/uuid"
)

type Config struct {
	Listen string `mapstructure:"listen"`
	Cert   string `mapstructure:"cert"`
	Key    string `mapstructure:"key"`
}

type ConnService interface {
	HandleReverseConn(ctx context.Context, conn net.Conn) error
}

type RevServer struct {
	connService ConnService
	cert        *tls.Certificate
	listen      string
}

func New(cfg *Config, connService ConnService) (*RevServer, error) {
	if cfg.Listen == "" {
		return nil, fmt.Errorf("listen address is required")
	}

	if (cfg.Cert != "" && cfg.Key == "") || (cfg.Cert == "" && cfg.Key != "") {
		return nil, fmt.Errorf("both cert and key are required for TLS")
	}

	var cert *tls.Certificate

	if cfg.Cert != "" {
		c, err := tls.LoadX509KeyPair(cfg.Cert, cfg.Key)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS certificate: %w", err)
		}

		cert = &c
	}

	return &RevServer{
		connService: connService,
		listen:      cfg.Listen,
		cert:        cert,
	}, nil
}

func (r *RevServer) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		l   net.Listener
		err error
	)

	if r.cert != nil {
		l, err = tls.Listen("tcp", r.listen, &tls.Config{
			Certificates: []tls.Certificate{*r.cert},
			MinVersion:   tls.VersionTLS13,
		})
	} else {
		l, err = net.Listen("tcp", r.listen)
	}

	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	go func() {
		<-ctx.Done()

		if err := l.Close(); err != nil {
			slog.ErrorContext(ctx, "failed to close listener", slog.Any("error", err))
		}
	}()

	wg := sync.WaitGroup{}

	defer wg.Wait()

	for {
		conn, err := l.Accept()
		if err != nil {
			return fmt.Errorf("failed to accept connection: %w", err)
		}

		wg.Add(1)

		go func() {
			defer wg.Done()
			defer func() { _ = conn.Close() }()

			//nolint:staticcheck,revive // don't want to couple with cmd package for now
			ctx := context.WithValue(ctx, "req_id", uuid.New().String())
			ctx, cancel := context.WithCancel(ctx)

			defer cancel()

			if err := r.connService.HandleReverseConn(ctx, conn); err != nil {
				slog.ErrorContext(ctx, "failed to handle connection", slog.Any("error", err))
			}
		}()
	}
}
