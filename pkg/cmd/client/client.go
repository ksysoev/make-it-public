package client

import (
	"context"

	"github.com/ksysoev/make-it-public/pkg/core"
	"github.com/spf13/cobra"
)

func InitCommand() cobra.Command {
	return cobra.Command{
		Use:   "client",
		Short: "Run client",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return RunClientCommand(cmd.Context())
		},
	}
}

func RunClientCommand(ctx context.Context) error {
	client := core.NewClientServer("localhost:8081")

	return client.Run(ctx)
}
