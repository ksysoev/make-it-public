// The MIT Server Management API

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"log/slog"
)

type Config struct {
	Listen string `mapstructure:"listen"`
}

type API struct {
	config Config
}

type Endpoint string

const (
	HealthCheckEndpoint Endpoint = "/health"
)

func New(cfg Config) *API {
	return &API{
		config: cfg,
	}
}

// Runs the API management server
func (api *API) Run(ctx context.Context) error {
	http.HandleFunc((string(HealthCheckEndpoint)), api.healthCheckHandler)
	server := &http.Server{
		Addr:              api.config.Listen,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
	}

	go func() {
		<-ctx.Done()

		_ = server.Close()
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}

	return nil
}

// healthCheckHandler returns the API status.
// This handler can be later modified to cross check required resources
func (api *API) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{"status": "healthy"}
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		slog.Error("Failed to encode response", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
	return
}
