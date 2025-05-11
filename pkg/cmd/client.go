package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ksysoev/make-it-public/pkg/core/token"
	"github.com/ksysoev/make-it-public/pkg/revclient"
)

func RunClientCommand(ctx context.Context, args *args) error {
	if err := initLogger(args); err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}

	tkn, err := token.Decode(args.Token)
	if err != nil {
		return fmt.Errorf("invalid token: %w", err)
	}

	cfg := revclient.Config{
		ServerAddr: args.Server,
		DestAddr:   args.Expose,
		NoTLS:      args.NoTLS,
		Insecure:   args.Insecure,
	}

	revcli := revclient.NewClientServer(cfg, tkn)

	slog.InfoContext(ctx, "revclient started", "server", args.Server)

	return revcli.Run(ctx)
}
