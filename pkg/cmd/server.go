package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ksysoev/make-it-public/pkg/api"
	"github.com/ksysoev/make-it-public/pkg/core"
	"github.com/ksysoev/make-it-public/pkg/edge"
	"github.com/ksysoev/make-it-public/pkg/repo/auth"
	"github.com/ksysoev/make-it-public/pkg/repo/connmng"
	"github.com/ksysoev/make-it-public/pkg/revproxy"
	"github.com/ksysoev/make-it-public/pkg/tcpedge"
	"golang.org/x/sync/errgroup"
)

// RunServerCommand initializes and starts both reverse proxy and HTTP servers for handling revclient connections.
// It takes ctx of type context.Context for managing the server lifecycle and args of type *args to load configuration.
// It returns an error if the configuration fails to load, servers cannot start, or any runtime error occurs.
func RunServerCommand(ctx context.Context, args *args) error {
	if err := initLogger(args); err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}

	cfg, err := loadConfig(args)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	authRepo := auth.New(&cfg.Auth)

	// Create two separate connection managers for web and TCP connections
	webConnManager := connmng.New()
	tcpConnManager := connmng.New()

	connService := core.New(webConnManager, tcpConnManager, authRepo)
	apiServ := api.New(cfg.API, connService)

	revServ, err := revproxy.New(&cfg.RevProxy, connService)
	if err != nil {
		return fmt.Errorf("failed to create reverse proxy server: %w", err)
	}

	httpServ, err := edge.New(cfg.HTTP, connService)
	if err != nil {
		return fmt.Errorf("failed to create http server: %w", err)
	}

	tcpEnabled := cfg.TCP.PortRange.Min > 0 && cfg.TCP.PortRange.Max > 0

	var tcpServ *tcpedge.TCPServer

	if tcpEnabled {
		tcpServ, err = tcpedge.New(cfg.TCP, connService)
		if err != nil {
			return fmt.Errorf("failed to create TCP edge server: %w", err)
		}
	}

	logAttrs := []any{
		"http", cfg.HTTP.Listen,
		"rev", cfg.RevProxy.Listen,
		"api", cfg.API.Listen,
	}

	if tcpEnabled {
		logAttrs = append(logAttrs, "tcp_port_range", fmt.Sprintf("%d-%d", cfg.TCP.PortRange.Min, cfg.TCP.PortRange.Max))
	} else {
		logAttrs = append(logAttrs, "tcp", "disabled")
	}

	slog.InfoContext(ctx, "server started", logAttrs...)

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error { return revServ.Run(ctx) })
	eg.Go(func() error { return httpServ.Run(ctx) })
	eg.Go(func() error { return apiServ.Run(ctx) })

	if tcpEnabled {
		eg.Go(func() error { return tcpServ.Run(ctx) })
	}

	return eg.Wait()
}
