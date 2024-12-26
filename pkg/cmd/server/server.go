package server

import (
	"context"
	"log/slog"

	"github.com/ksysoev/make-it-public/pkg/core"
	"github.com/spf13/cobra"
)

type flags struct {
	httpListen string
	revListen  string
}

func InitCommand() cobra.Command {
	args := flags{}

	cmd := cobra.Command{
		Use:   "server",
		Short: "Run server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return RunServerCommand(cmd.Context(), &args)
		},
	}

	cmd.Flags().StringVar(&args.httpListen, "http-listen", ":8080", "HTTP server listen address")
	cmd.Flags().StringVar(&args.revListen, "rev-listen", ":8081", "Reverse server listen address")

	return cmd
}

func RunServerCommand(ctx context.Context, args *flags) error {
	revServ := core.NewRevServer(args.revListen)

	if err := revServ.Start(ctx); err != nil {
		return err
	}

	httpServ := core.NewHTTPServer(args.httpListen, revServ)

	slog.InfoContext(ctx, "server started", "http", args.httpListen, "rev", args.revListen)

	return httpServ.Run(ctx)
}
