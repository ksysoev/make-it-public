package client

import (
	"context"

	"github.com/ksysoev/make-it-public/pkg/core"
	"github.com/spf13/cobra"
)

type flags struct {
	server string
	port   string
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
	cmd.Flags().StringVar(&args.port, "port", "8080", "port to expose")

	return cmd
}

func RunClientCommand(ctx context.Context, args *flags) error {
	client := core.NewClientServer(args.server)

	return client.Run(ctx)
}
