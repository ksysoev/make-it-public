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

// InitCommand initializes and returns a cobra.Command for running the server with configurable args.
func InitCommand() cobra.Command {
	arg := args{}

	cmd := cobra.Command{
		Use:   "mitserver",
		Short: "Make It Public server",
		Long:  "Make It Public server is a reverse proxy server that allows you to expose your local services to the internet.",
	}

	cmd.AddCommand(InitServerCommand(&arg))
	cmd.AddCommand(InitTokenCommand(&arg))

	cmd.Flags().StringVar(&arg.configPath, "config", "runtime/config.yaml", "config path")
	cmd.Flags().StringVar(&arg.logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	cmd.Flags().BoolVar(&arg.textFormat, "log-text", false, "log in text format, otherwise JSON")

	return cmd
}

func InitServerCommand(arg *args) *cobra.Command {
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

func InitTokenCommand(arg *args) *cobra.Command {
	cmd := cobra.Command{
		Use:   "token",
		Short: "",
		Long:  "",
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
