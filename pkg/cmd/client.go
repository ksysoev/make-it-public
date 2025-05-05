package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ksysoev/make-it-public/pkg/core/token"
	"github.com/ksysoev/make-it-public/pkg/revclient"
)

func RunClientCommand(ctx context.Context, args *args) error {
	tkn, err := token.Decode(args.token)
	if err != nil {
		return fmt.Errorf("invalid token: %w", err)
	}

	revcli := revclient.NewClientServer(args.server, args.expose, tkn)

	slog.InfoContext(ctx, "revclient started", "server", args.server)

	return revcli.Run(ctx)
}
