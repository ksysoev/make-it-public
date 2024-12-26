package server

import (
	"context"

	"github.com/ksysoev/make-it-public/pkg/core"
	"github.com/spf13/cobra"
)

func InitCommand() cobra.Command {
	return cobra.Command{
		Use:   "server",
		Short: "Run server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return RunServerCommand(cmd.Context())
		},
	}
}

func RunServerCommand(ctx context.Context) error {
	revServ := core.NewRevServer(":8081")

	if err := revServ.Start(ctx); err != nil {
		return err
	}

	httpServ := core.NewHTTPServer(":8080", revServ)

	return httpServ.Run(ctx)
}
