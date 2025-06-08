package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ksysoev/make-it-public/pkg/core/token"
	"github.com/ksysoev/make-it-public/pkg/dummy"
	"github.com/ksysoev/make-it-public/pkg/revclient"

	"golang.org/x/sync/errgroup"
)

func RunClientCommand(ctx context.Context, args *args) error {
	if err := initLogger(args); err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}

	tkn, err := token.Decode(args.Token)
	if err != nil {
		return fmt.Errorf("invalid token: %w", err)
	}

	exposeAddr := args.Expose

	eg, ctx := errgroup.WithContext(ctx)
	if exposeAddr == "" && args.LocalServer {
		lclSrv := dummy.New()
		eg.Go(func() error { return lclSrv.Run(ctx) })
		exposeAddr = lclSrv.Addr()
	}

	cfg := revclient.Config{
		ServerAddr: args.Server,
		DestAddr:   exposeAddr,
		NoTLS:      args.NoTLS,
		Insecure:   args.Insecure,
	}

	revcli := revclient.NewClientServer(cfg, tkn)

	slog.InfoContext(ctx, "revclient started", "server", args.Server)
	eg.Go(func() error { return revcli.Run(ctx) })

	return eg.Wait()
}
