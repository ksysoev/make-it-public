package server

import (
	"github.com/spf13/cobra"
)

type args struct {
	configPath string
	logLevel   string
	version    string
	textFormat bool
}

// InitCommand creates and initializes the root command for the Make It Public server CLI application.
// It sets up subcommands for starting the server and generating tokens, adds command-line flag definitions,
// and returns the fully configured cobra.Command instance for execution.
func InitCommand() cobra.Command {
	arg := args{}

	cmd := cobra.Command{
		Use:   "mitserver",
		Short: "Make It Public server",
		Long:  "Make It Public server is a reverse proxy server that allows you to expose your local services to the internet.",
	}

	cmd.AddCommand(InitServeCommand(&arg))
	cmd.AddCommand(InitTokenCommand(&arg))

	cmd.Flags().StringVar(&arg.configPath, "config", "runtime/config.yaml", "config path")
	cmd.Flags().StringVar(&arg.logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	cmd.Flags().BoolVar(&arg.textFormat, "log-text", false, "log in text format, otherwise JSON")

	return cmd
}

// InitServeCommand creates and initializes the "serve" command for starting the server and its subcommands.
// It utilizes the provided args parameter to configure options like configuration path and log level.
// Returns a pointer to a cobra.Command that includes the "all" subcommand to run all server modules.
// Returns nil if the args parameter is not properly initialized.
func InitServeCommand(arg *args) *cobra.Command {
	cmd := cobra.Command{
		Use:   "serve",
		Short: "Run the server",
		Long:  "Run the server with the specified configuration.",
	}

	cmdRunAll := &cobra.Command{
		Use:   "all",
		Short: "Run all servers",
		Long:  "Run all servers, including reverse proxy, HTTP server, and API server.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if err := RunServerCommand(ctx, arg); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.AddCommand(cmdRunAll)

	return &cmd
}

// InitTokenCommand initializes the "token" command with subcommands like "generate" for token management in the CLI.
// It binds the "generate" subcommand to trigger token generation using the provided configuration arguments.
// Accepts arg which contains configuration details like config path, log level, and text format.
// Returns a pointer to a cobra.Command that encapsulates the "token" command and its subcommands.
func InitTokenCommand(arg *args) *cobra.Command {
	cmd := cobra.Command{
		Use:   "token",
		Short: "Token management",
		Long:  "Token management commands for the server.",
	}

	cmdGenerateToken := &cobra.Command{
		Use:   "generate",
		Short: "Generate a new token",
		Long:  "Generate a new token for authentication.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if err := RunGenerateToken(ctx, arg); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.AddCommand(cmdGenerateToken)

	return &cmd
}
