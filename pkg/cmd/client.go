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
		if args.Status < 200 || args.Status >= 600 {
			return fmt.Errorf("invalid status code: %d", args.Status)
		}

		resp := dummy.Response{
			Status: args.Status,
		}

		switch {
		case args.JSONResponse != "":
			resp.Body = args.JSONResponse
			resp.ContentType = "application/json"
		case args.Response != "":
			resp.Body = args.Response
			resp.ContentType = "text/plain"
		}

		lclSrv := dummy.New(resp)

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

	slog.InfoContext(ctx, "mit client started", "server", args.Server)
	eg.Go(func() error { return revcli.Run(ctx) })

	return eg.Wait()
}
