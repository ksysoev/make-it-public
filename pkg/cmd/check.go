package cmd

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// RunHealthCheck performs a health check on the local server's API endpoint.
// It initializes the logger and loads the configuration. It requires the API listen address to be properly configured.
// Accepts ctx for managing the lifecycle and arg for configuration options.
// Returns an error if the logger fails to initialize, the configuration cannot be loaded, the API address is invalid,
// or the health check HTTP request fails, including non-200 HTTP response statuses.
func RunHealthCheck(_ context.Context, arg *args) error {
	if err := initLogger(arg); err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}

	cfg, err := loadConfig(arg)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	apiListen := cfg.API.Listen
	if apiListen == "" {
		return fmt.Errorf("API listen address is not configured")
	}

	addrParts := strings.Split(apiListen, ":")
	if len(addrParts) != 2 {
		return fmt.Errorf("invalid API listen address format: %s", apiListen)
	}

	port := addrParts[1]

	cl := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := cl.Get(fmt.Sprintf("http://localhost:%s/health", port))
	if err != nil {
		return fmt.Errorf("failed to perform health check: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed with status code: %d", resp.StatusCode)
	}

	return nil
}
