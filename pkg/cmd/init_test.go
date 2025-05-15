package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitCommand(t *testing.T) {
	cmd := InitCommand(BuildInfo{})

	assert.Equal(t, "mit", cmd.Use)
	assert.Contains(t, cmd.Short, "Make It Public")
	assert.Contains(t, cmd.Long, "Make It Public Reverse Connect Proxy is a tool for exposing local services to the internet.")

	require.Len(t, cmd.Commands(), 1)
	assert.Equal(t, "server", cmd.Commands()[0].Use)
}

func TestInitRunCommand(t *testing.T) {
	arg := &args{}
	cmd := initRunCommand(arg)

	assert.Equal(t, "run", cmd.Use)
	assert.Contains(t, cmd.Short, "Run the server")
	assert.Contains(t, cmd.Long, "specified configuration")

	require.Len(t, cmd.Commands(), 1)
	assert.Equal(t, "all", cmd.Commands()[0].Use)
	assert.Contains(t, cmd.Commands()[0].Short, "Run all server components")
}

func TestInitTokenCommand(t *testing.T) {
	arg := &args{}
	cmd := initTokenCommand(arg)

	assert.Equal(t, "token", cmd.Use)
	assert.Contains(t, cmd.Short, "Token management")
	assert.Contains(t, cmd.Long, "commands for the server")

	require.Len(t, cmd.Commands(), 1)
	generateCmd := cmd.Commands()[0]
	assert.Equal(t, "generate", generateCmd.Use)
	assert.Contains(t, generateCmd.Short, "Generate a new token")

	// Validate flags of the generate subcommand
	keyIDFlag := generateCmd.Flags().Lookup("key-id")
	require.NotNil(t, keyIDFlag)
	assert.Equal(t, "", keyIDFlag.DefValue)
	assert.Contains(t, keyIDFlag.Usage, "Key ID for the token")

	ttlFlag := generateCmd.Flags().Lookup("ttl")
	require.NotNil(t, ttlFlag)
	assert.Equal(t, "1", ttlFlag.DefValue)
	assert.Contains(t, ttlFlag.Usage, "Token time to live in hours")
}
