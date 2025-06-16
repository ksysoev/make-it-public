package revproxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"github.com/google/uuid"
	"github.com/ksysoev/make-it-public/pkg/watcher"
)

type Config struct {
	Listen string `mapstructure:"listen"`
	Cert   string `mapstructure:"cert"`
	Key    string `mapstructure:"key"`
}

type ConnService interface {
	HandleReverseConn(ctx context.Context, conn net.Conn) error
}

type Certificate struct {
	Cert         *tls.Certificate
	CertFilePath string
	KeyFilePath  string
}

//nolint:govet // linter mistakes Mutex to be smaller
type RevServer struct {
	certMu      sync.RWMutex
	listen      string
	connService ConnService
	cert        *Certificate
	certWatcher *watcher.FileWatcher
}

func New(cfg *Config, connService ConnService) (*RevServer, error) {
	if cfg.Listen == "" {
		return nil, fmt.Errorf("listen address is required")
	}

	if (cfg.Cert != "" && cfg.Key == "") || (cfg.Cert == "" && cfg.Key != "") {
		return nil, fmt.Errorf("both cert and key are required for TLS")
	}

	var (
		cert        *Certificate
		certWatcher *watcher.FileWatcher
	)

	if cfg.Cert != "" {
		c, err := tls.LoadX509KeyPair(cfg.Cert, cfg.Key)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS certificate: %w", err)
		}

		cert = &Certificate{
			Cert:         &c,
			CertFilePath: cfg.Cert,
			KeyFilePath:  cfg.Key,
		}

		certWatcher, err = watcher.NewFileWatcher(cfg.Cert)

		if err != nil {
			return nil, fmt.Errorf("failed to create file watcher for TLS certificate: %w", err)
		}
	}

	return &RevServer{
		connService: connService,
		listen:      cfg.Listen,
		cert:        cert,
		certWatcher: certWatcher,
		certMu:      sync.RWMutex{},
	}, nil
}

func (r *RevServer) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	wg := sync.WaitGroup{}

	if r.cert != nil {
		subscriber := r.certWatcher.Subscribe()
		defer r.certWatcher.Unsubscribe(subscriber)

		wg.Add(1)

		go func() {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				case notification := <-subscriber:
					slog.InfoContext(ctx, "TLS certificate file changed", slog.String("path", notification.Path))

					newCert, err := tls.LoadX509KeyPair(r.cert.CertFilePath, r.cert.KeyFilePath)
					if err != nil {
						slog.ErrorContext(ctx, "failed to reload TLS certificate", slog.Any("error", err))
						continue
					}

					r.certMu.Lock()

					r.cert = &Certificate{
						Cert:         &newCert,
						CertFilePath: r.cert.CertFilePath,
						KeyFilePath:  r.cert.KeyFilePath}

					slog.InfoContext(ctx, "TLS certificate reloaded successfully")

					r.certMu.Unlock()
				}
			}
		}()
	}

	var (
		l   net.Listener
		err error
	)

	if r.cert != nil {
		r.certMu.Lock()
		l, err = tls.Listen("tcp", r.listen, &tls.Config{
			Certificates: []tls.Certificate{*r.cert.Cert},
			MinVersion:   tls.VersionTLS13,
		})
		r.certMu.Unlock()
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
