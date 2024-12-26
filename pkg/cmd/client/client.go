package client

import (
	"context"

	"github.com/ksysoev/make-it-public/pkg/core"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slog"
)

type flags struct {
	server string
	expose string
}

func InitCommand() cobra.Command {
	args := flags{}

	cmd := cobra.Command{
		Use:   "client",
		Short: "Run client",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return RunClientCommand(cmd.Context(), &args)
		},
	}

	cmd.Flags().StringVar(&args.server, "server", "localhost:8081", "server address")
	cmd.Flags().StringVar(&args.expose, "expose", "localhost:80", "expose service")

	return cmd
}

func RunClientCommand(ctx context.Context, args *flags) error {
	client := core.NewClientServer(args.server, args.expose)

	slog.InfoContext(ctx, "client started", "server", args.server)

	return client.Run(ctx)
}
