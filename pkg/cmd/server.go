package cmd

import (
	"context"

	"github.com/ksysoev/make-it-public/pkg/core"
)

func RunServerCommand(ctx context.Context) error {
	revServ := core.NewRevServer(":8081")
	if err := revServ.Start(ctx); err != nil {
		return err
	}

	httpServ := core.NewHTTPServer(":8080", revServ)
	return httpServ.Run(ctx)
}
