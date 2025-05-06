package cmd

import (
	"github.com/spf13/cobra"
)

type args struct {
	// client args
	server string
	expose string
	token  string

	// server args
	configPath string
	logLevel   string
	version    string
	textFormat bool
}

// InitCommand initializes the root command of the CLI application with its subcommands and flags.
// It sets up the "mit" command with pre-defined subcommands, including the "server" command.
// Returns a cobra.Command configured with flags for setting server address, service exposure, and token authentication.
func InitCommand() cobra.Command {
	arg := args{}

	cmd := cobra.Command{
		Use:   "mit",
		Short: "Make It Public Reverse Connect Proxy",
		Long:  "Make It Public Reverse Connect Proxy is a tool for exposing local services to the internet.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return RunClientCommand(cmd.Context(), &arg)
		},
	}

	cmd.Flags().StringVar(&arg.server, "server", "localhost:8081", "server address")
	cmd.Flags().StringVar(&arg.expose, "expose", "localhost:80", "expose service")
	cmd.Flags().StringVar(&arg.token, "token", "", "token")

	cmd.AddCommand(initServerCommand(&arg))

	return cmd
}

// initServerCommand initializes the "server" command for the CLI application, adding necessary flags and subcommands.
// It configures the command with options for specifying the configuration file, log level, and log format.
// Accepts arg of type *args to set up custom behavior and flag bindings.
// Returns a pointer to the fully initialized cobra.Command that includes subcommands for running the server and managing tokens.
func initServerCommand(arg *args) *cobra.Command {
	cmd := cobra.Command{
		Use:   "server",
		Short: "Make It Public Reverse Connect Proxy Server",
		Long:  "Make It Public Reverse Connect Proxy Server is a service for exposing local services to the internet.",
	}

	cmd.PersistentFlags().StringVar(&arg.configPath, "config", "runtime/config.yaml", "config path")
	cmd.PersistentFlags().StringVar(&arg.logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	cmd.PersistentFlags().BoolVar(&arg.textFormat, "log-text", false, "log in text format, otherwise JSON")

	cmd.AddCommand(initRunCommand(arg))
	cmd.AddCommand(initTokenCommand(arg))

	return &cmd
}

// initRunCommand initializes the "run" command and its subcommands for starting the server.
// It configures the main "run" command as well as the "all" subcommand to start all servers.
// Accepts arg of type *args to configure command behavior and set flags.
// Returns a pointer to the initialized cobra.Command for execution.
func initRunCommand(arg *args) *cobra.Command {
	cmd := cobra.Command{
		Use:   "run",
		Short: "Run the server",
		Long:  "Run the server with the specified configuration.",
	}

	cmdRunAll := &cobra.Command{
		Use:   "all",
		Short: "Run all server components",
		Long:  "Run all servers, including reverse proxy, HTTP server, and API server.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return RunServerCommand(cmd.Context(), arg)
		},
	}

	cmd.AddCommand(cmdRunAll)

	return &cmd
}

// initTokenCommand creates and configures the "token" command with subcommands for token management.
// It accepts an argument structure containing configuration and state details required by the command.
// Returns a pointer to the initialized cobra.Command.
func initTokenCommand(arg *args) *cobra.Command {
	cmd := cobra.Command{
		Use:   "token",
		Short: "Token management",
		Long:  "Token management commands for the server.",
	}

	keyID := ""

	cmdGenerateToken := &cobra.Command{
		Use:   "generate",
		Short: "Generate a new token",
		Long:  "Generate a new token for authentication.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return RunGenerateToken(cmd.Context(), arg, keyID)
		},
	}

	cmdGenerateToken.Flags().StringVar(&keyID, "key-id", "", "Key ID for the token")

	cmd.AddCommand(cmdGenerateToken)

	return &cmd
}
