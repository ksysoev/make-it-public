package cmd

import (
	"context"

	"github.com/ksysoev/make-it-public/pkg/core"
)

func RunClientCommand(ctx context.Context) error {
	client := core.NewClientServer("localhost:8081")

	return client.Run(ctx)
}
