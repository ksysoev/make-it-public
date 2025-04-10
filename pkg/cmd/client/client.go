package client

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ksysoev/make-it-public/pkg/core/token"
	"github.com/ksysoev/make-it-public/pkg/revclient"
	"github.com/spf13/cobra"
)

type flags struct {
	server string
	expose string
	token  string
}

func InitCommand() cobra.Command {
	args := flags{}

	cmd := cobra.Command{
		Use:   "revclient",
		Short: "Run revclient",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return RunClientCommand(cmd.Context(), &args)
		},
	}

	cmd.Flags().StringVar(&args.server, "server", "localhost:8081", "server address")
	cmd.Flags().StringVar(&args.expose, "expose", "localhost:80", "expose service")
	cmd.Flags().StringVar(&args.token, "token", "", "token")

	return cmd
}

func RunClientCommand(ctx context.Context, args *flags) error {
	tkn, err := token.Decode(args.token)
	if err != nil {
		return fmt.Errorf("invalid token: %w", err)
	}

	revcli := revclient.NewClientServer(args.server, args.expose, tkn)

	slog.InfoContext(ctx, "revclient started", "server", args.server)

	return revcli.Run(ctx)
}
