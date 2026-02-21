package tcpedge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate_Valid(t *testing.T) {
	cfg := Config{
		ListenHost: "0.0.0.0",
		Public:     PublicConfig{Host: "example.com"},
		PortRange:  PortRange{Min: 10000, Max: 20000},
	}

	require.NoError(t, cfg.Validate())
}

func TestConfig_Validate_EmptyListenHost(t *testing.T) {
	cfg := Config{
		ListenHost: "",
		Public:     PublicConfig{Host: "example.com"},
		PortRange:  PortRange{Min: 10000, Max: 20000},
	}

	assert.Error(t, cfg.Validate())
}

func TestConfig_Validate_EmptyPublicHost(t *testing.T) {
	cfg := Config{
		ListenHost: "0.0.0.0",
		Public:     PublicConfig{Host: ""},
		PortRange:  PortRange{Min: 10000, Max: 20000},
	}

	assert.Error(t, cfg.Validate())
}

func TestConfig_Validate_MinBelowPrivilegedBoundary(t *testing.T) {
	cfg := Config{
		ListenHost: "0.0.0.0",
		Public:     PublicConfig{Host: "example.com"},
		PortRange:  PortRange{Min: 80, Max: 20000},
	}

	assert.Error(t, cfg.Validate())
}

func TestConfig_Validate_MaxAbove65535(t *testing.T) {
	cfg := Config{
		ListenHost: "0.0.0.0",
		Public:     PublicConfig{Host: "example.com"},
		PortRange:  PortRange{Min: 10000, Max: 70000},
	}

	assert.Error(t, cfg.Validate())
}

func TestConfig_Validate_MinGreaterThanMax(t *testing.T) {
	cfg := Config{
		ListenHost: "0.0.0.0",
		Public:     PublicConfig{Host: "example.com"},
		PortRange:  PortRange{Min: 20000, Max: 10000},
	}

	assert.Error(t, cfg.Validate())
}

func TestConfig_Validate_MinEqualsMax(t *testing.T) {
	cfg := Config{
		ListenHost: "0.0.0.0",
		Public:     PublicConfig{Host: "example.com"},
		PortRange:  PortRange{Min: 10000, Max: 10000},
	}

	assert.Error(t, cfg.Validate())
}
