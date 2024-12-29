package server

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ksysoev/make-it-public/pkg/edge"
	"github.com/ksysoev/make-it-public/pkg/revproxy"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

type flags struct {
	configPath string
}

// InitCommand initializes and returns a cobra.Command for running the server with configurable flags.
func InitCommand() cobra.Command {
	args := flags{}

	cmd := cobra.Command{
		Use:   "server",
		Short: "Run server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return RunServerCommand(cmd.Context(), &args)
		},
	}

	cmd.Flags().StringVar(&args.configPath, "config", "runtime/config.yaml", "config path")

	return cmd
}

// RunServerCommand initializes and starts the server using the provided context and configuration flags.
// It loads the application configuration, initializes reverse and HTTP servers, and starts them concurrently.
func RunServerCommand(ctx context.Context, args *flags) error {
	cfg, err := loadConfig(args)
	if err != nil {
		return fmt.Errorf("failed to loag config: %w", err)
	}

	revServ := revproxy.New(cfg.RevProxy.Listen, cfg.RevProxy.Users)
	httpServ := edge.NewHTTPServer(cfg.HTTP.Listen, revServ)

	slog.InfoContext(ctx, "server started", "http", cfg.HTTP.Listen, "rev", cfg.RevProxy.Listen)

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error { return revServ.Run(ctx) })
	eg.Go(func() error { return httpServ.Run(ctx) })

	return eg.Wait()
}
