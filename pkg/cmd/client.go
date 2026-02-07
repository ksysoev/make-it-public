package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ksysoev/make-it-public/pkg/core/token"
	"github.com/ksysoev/make-it-public/pkg/display"
	"github.com/ksysoev/make-it-public/pkg/dummy"
	"github.com/ksysoev/make-it-public/pkg/revclient"

	"golang.org/x/sync/errgroup"
)

func RunClientCommand(ctx context.Context, args *args) error {
	// Initialize display for terminal output
	disp := display.New(args.Interactive)

	if err := initLogger(args); err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}

	// Validate token with improved error messaging
	tkn, err := token.Decode(args.Token)
	if err != nil {
		disp.ShowError("Invalid token", err,
			"Get a token from your administrator or generate one with:\n"+
				"  mit server token generate --key-id <name>")

		return fmt.Errorf("invalid token: %w", err)
	}

	exposeAddr := args.Expose
	eg, ctx := errgroup.WithContext(ctx)

	if exposeAddr == "" && args.LocalServer {
		lclSrv, err := dummy.New(dummy.Config{
			Status:      args.Status,
			JSON:        args.JSON,
			Body:        args.Body,
			Headers:     args.Headers,
			Interactive: args.Interactive,
		})
		if err != nil {
			disp.ShowError("Failed to create local server", err, "")
			return fmt.Errorf("failed to create local server: %w", err)
		}

		eg.Go(func() error { return lclSrv.Run(ctx) })

		exposeAddr = lclSrv.Addr()
	}

	// Validate that we have something to expose
	if exposeAddr == "" {
		disp.ShowError("No service to expose", nil,
			"Specify a local service with --expose or use --dummy for testing:\n"+
				"  mit --token <token> --expose localhost:8080\n"+
				"  mit --token <token> --dummy")

		return fmt.Errorf("no service to expose: use --expose or --dummy flag")
	}

	cfg := revclient.Config{
		ServerAddr: args.Server,
		DestAddr:   exposeAddr,
		NoTLS:      args.NoTLS,
		Insecure:   args.Insecure,
		EnableV2:   !args.DisableV2, // V2 enabled by default, use --disable-v2 for old servers
	}

	// Start spinner while connecting
	spinner := disp.ShowConnecting(args.Server)
	if spinner != nil {
		defer spinner.Stop()
	}

	// Create client with callbacks for display
	revcli := revclient.NewClientServer(cfg, tkn,
		revclient.WithOnConnected(func(url string) {
			// Stop spinner and show success banner
			if spinner != nil {
				spinner.Success("Connected!")
			}

			disp.ShowConnected(url, exposeAddr)
		}),
		revclient.WithOnRequest(func(clientIP string) {
			// Show request separator for each incoming connection
			disp.ShowRequestSeparator(clientIP)
		}),
	)

	slog.InfoContext(ctx, "mit client started", "server", args.Server)
	eg.Go(func() error { return revcli.Run(ctx) })

	err = eg.Wait()
	if err != nil {
		// Stop spinner if still running (connection failed)
		if spinner != nil {
			spinner.Fail("Connection failed")
		}
	}

	return err
}
