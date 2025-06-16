package cmd

import (
	"context"
	"log/slog"
	"os"
)

// ContextHandler is a custom slog.Handler that enriches log records with application-specific attributes.
// It embeds a slog.Handler and adds attributes like application name and version, as well as request-specific context data.
type ContextHandler struct {
	slog.Handler
	ver string
	app string
}

// Handle processes a log record by enriching it with context and application-specific attributes.
// It adds attributes such as "req_id" from the context, "app", and "ver" before delegating to the embedded handler.
// Returns error if the embedded handler fails.

//nolint:gocritic // ignore this linting rule
func (h ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if requestID, ok := ctx.Value("req_id").(string); ok {
		r.AddAttrs(slog.String("req_id", requestID))
	}

	r.AddAttrs(slog.String("app", h.app), slog.String("ver", h.ver))

	return h.Handler.Handle(ctx, r)
}

// initLogger initializes the default logger for the application using slog.
// It does not take any parameters.
// It returns an error if the logger initialization fails, although in this implementation, it always returns nil.
func initLogger(arg *args) error {
	var logLevel slog.Level
	if err := logLevel.UnmarshalText([]byte(arg.LogLevel)); err != nil {
		return err
	}

	options := &slog.HandlerOptions{
		Level:       logLevel,
		ReplaceAttr: createReplacer(arg),
	}

	var logHandler slog.Handler
	if arg.TextFormat {
		logHandler = slog.NewTextHandler(os.Stdout, options)
	} else {
		logHandler = slog.NewJSONHandler(os.Stdout, options)
	}

	ctxHandler := &ContextHandler{
		Handler: logHandler,
		ver:     arg.Version,
		app:     "mit",
	}

	logger := slog.New(ctxHandler)

	slog.SetDefault(logger)

	return nil
}

// createReplacer creates a function to selectively modify log attributes when the application is in interactive mode.
// It disables attributes like "time", "app", "ver", and "level" if the arg.Interactive flag is true.
// Accepts arg configuration options for interactive mode.
// Returns a function that filters out specific log attributes or nil if interactive mode is disabled.
func createReplacer(arg *args) func(group []string, a slog.Attr) slog.Attr {
	if !arg.Interactive {
		return nil
	}

	return func(_ []string, a slog.Attr) slog.Attr {
		switch a.Key {
		case "time", "app", "ver", "level":
			return slog.Attr{}
		default:
			return a
		}
	}
}
