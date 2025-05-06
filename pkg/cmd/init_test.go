package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitCommand(t *testing.T) {
	cmd := InitCommand()

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
	assert.Equal(t, "generate", cmd.Commands()[0].Use)
	assert.Contains(t, cmd.Commands()[0].Short, "Generate a new token")
}
