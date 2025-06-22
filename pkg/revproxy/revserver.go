package revproxy

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"github.com/google/uuid"
	"github.com/ksysoev/make-it-public/pkg/revproxy/watcher"
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
	CertPath string
	KeyPath  string
}

type RevServer struct {
	connService ConnService
	cert        *Certificate
	listen      string
}

func New(cfg *Config, connService ConnService) (*RevServer, error) {
	if cfg.Listen == "" {
		return nil, fmt.Errorf("listen address is required")
	}

	if (cfg.Cert != "" && cfg.Key == "") || (cfg.Cert == "" && cfg.Key != "") {
		return nil, fmt.Errorf("both cert and key are required for TLS")
	}

	srv := &RevServer{
		connService: connService,
		listen:      cfg.Listen,
	}

	if cfg.Cert != "" && cfg.Key != "" {
		srv.cert = &Certificate{
			CertPath: cfg.Cert,
			KeyPath:  cfg.Key,
		}
	}

	return srv, nil
}

func (r *RevServer) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		l   net.Listener
		err error
	)

	if r.cert != nil {
		cert, errCert := loadTLSCertificate(ctx, r.cert.CertPath, r.cert.KeyPath, func() {
			slog.InfoContext(ctx, "TLS certificate is updated, restarting service", slog.String("cert", r.cert.CertPath), slog.String("key", r.cert.KeyPath))
			cancel() // Cancel the context to stop the current listener
		})
		if errCert != nil {
			return fmt.Errorf("failed to load TLS certificate: %w", err)
		}

		l, err = tls.Listen("tcp", r.listen, &tls.Config{
			Certificates: []tls.Certificate{*cert},
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

	return r.processConnections(ctx, l)
}

// processConnections manages incoming network connections and delegates handling to the ConnService.
// It continuously accepts connections from the provided net.Listener, spawns a new goroutine to handle each connection,
// and waits for all spawned goroutines to complete on termination.
// Accepts ctx for operation context and listener l to listen for incoming connections.
// Returns an error if accepting connections fails or the listener is closed unexpectedly.
func (r *RevServer) processConnections(ctx context.Context, l net.Listener) error {
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

// loadTLSCertificate loads a TLS certificate and optionally monitors the specified files for changes to reload the certificate.
// It requires certFile and keyFile paths to load the certificate. The onUpdate callback is triggered on file changes if provided.
// Returns a pointer to the loaded tls.Certificate and an error if loading fails or file watcher creation fails.
func loadTLSCertificate(ctx context.Context, certFile, keyFile string, onUpdate func()) (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS certificate: %w", err)
	}

	if onUpdate != nil {
		w, err := watcher.NewFileWatcher(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create file watcher for TLS certificate: %w", err)
		}

		subscriber := w.Subscribe()
		go func() {
			defer w.Unsubscribe(subscriber)

			for {
				select {
				case <-ctx.Done():
					return
				case notification := <-subscriber:
					slog.DebugContext(ctx, "TLS certificate file changed", slog.String("path", notification.Path))

					_, err := tls.LoadX509KeyPair(certFile, keyFile)
					if err != nil {
						slog.ErrorContext(ctx, "failed to reload TLS certificate", slog.Any("error", err))
						continue
					}

					onUpdate()
				}
			}
		}()
	}

	return &cert, nil
}
