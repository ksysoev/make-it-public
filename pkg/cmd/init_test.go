package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitCommand(t *testing.T) {
	cmd := InitCommand()

	assert.Equal(t, "mitserver", cmd.Use)
	assert.Contains(t, cmd.Short, "Make It Public")
	assert.Contains(t, cmd.Long, "reverse proxy server")

	require.Len(t, cmd.Commands(), 2)
	assert.Equal(t, "serve", cmd.Commands()[0].Use)
	assert.Equal(t, "token", cmd.Commands()[1].Use)
}

func TestInitServeCommand(t *testing.T) {
	arg := &args{}
	cmd := InitServeCommand(arg)

	assert.Equal(t, "serve", cmd.Use)
	assert.Contains(t, cmd.Short, "Run the server")
	assert.Contains(t, cmd.Long, "specified configuration")

	require.Len(t, cmd.Commands(), 1)
	assert.Equal(t, "all", cmd.Commands()[0].Use)
	assert.Contains(t, cmd.Commands()[0].Short, "Run all servers")
}

func TestInitTokenCommand(t *testing.T) {
	arg := &args{}
	cmd := InitTokenCommand(arg)

	assert.Equal(t, "token", cmd.Use)
	assert.Contains(t, cmd.Short, "Token management")
	assert.Contains(t, cmd.Long, "commands for the server")

	require.Len(t, cmd.Commands(), 1)
	assert.Equal(t, "generate", cmd.Commands()[0].Use)
	assert.Contains(t, cmd.Commands()[0].Short, "Generate a new token")
}
