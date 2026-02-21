package tcpedge

import (
	"errors"
	"fmt"
)

// Config holds configuration for the TCP edge server.
type Config struct {
	ListenHost string       `mapstructure:"listen_host"`
	Public     PublicConfig `mapstructure:"public"`
	PortRange  PortRange    `mapstructure:"port_range"`
}

// PublicConfig defines the publicly advertised hostname for TCP endpoints.
type PublicConfig struct {
	Host string `mapstructure:"host"`
}

// PortRange defines the inclusive range of TCP ports available for allocation.
type PortRange struct {
	Min int `mapstructure:"min"`
	Max int `mapstructure:"max"`
}

// Validate checks that the Config is valid. It returns an error describing the
// first violation found.
func (c *Config) Validate() error {
	if c.ListenHost == "" {
		return errors.New("listen_host must not be empty")
	}

	if c.Public.Host == "" {
		return errors.New("public.host must not be empty")
	}

	if c.PortRange.Min < 1024 {
		return fmt.Errorf("port_range.min must be >= 1024, got %d", c.PortRange.Min)
	}

	if c.PortRange.Max > 65535 {
		return fmt.Errorf("port_range.max must be <= 65535, got %d", c.PortRange.Max)
	}

	if c.PortRange.Min > c.PortRange.Max {
		return fmt.Errorf("port_range.min (%d) must be <= port_range.max (%d)", c.PortRange.Min, c.PortRange.Max)
	}

	if c.PortRange.Min == c.PortRange.Max {
		return fmt.Errorf("port_range must contain at least 2 ports, got min==max==%d", c.PortRange.Min)
	}

	return nil
}
