// The MIT Server Management API

package api

import (
	"encoding/json"
	"net/http"

	"log/slog"
)

type API struct {
	Listen string `mapstructure:"listen"`
}

type Endpoint string

const (
	HealthCheckEndpoint Endpoint = "/health"
)

func New(listenAddr string) *API {
	return &API{
		Listen: listenAddr,
	}
}

// Runs the API management server
func (api *API) Run() error {
	http.HandleFunc((string(HealthCheckEndpoint)), api.healthCheckHandler)
	return http.ListenAndServe(api.Listen, nil)
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
