// The MIT Server Management API

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"log/slog"

	"github.com/google/uuid"
	"github.com/ksysoev/make-it-public/pkg/core/token"
)

type Config struct {
	Listen      string `mapstructure:"listen"`
	TokenExpiry uint   `mapstructure:"token_expiry"`
}

type API struct {
	config Config
}

type Endpoint string

const (
	HealthCheckEndpoint   Endpoint = "GET /health"
	GenerateTokenEndpoint Endpoint = "POST /generateToken"
)

func New(cfg Config) *API {
	return &API{
		config: cfg,
	}
}

// Runs the API management server
func (api *API) Run(ctx context.Context) error {
	router := http.NewServeMux()

	router.HandleFunc((string(HealthCheckEndpoint)), api.healthCheckHandler)
	router.HandleFunc((string(GenerateTokenEndpoint)), api.generateTokenHandler)

	server := &http.Server{
		Addr:              api.config.Listen,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
		Handler:           router,
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
func (api *API) healthCheckHandler(w http.ResponseWriter, _ *http.Request) {
	resp := map[string]string{"status": "healthy"}

	w.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(w).Encode(resp)

	if err != nil {
		slog.Error("Failed to encode response", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}
}

// generateTokenHandler is an endpoint to create API token.
// It optionally accepts a key ID, which is automatically generated if not provided.
// It also optionally accepts a TTL for API token, which is set to a default value if not provided.
// As a part of response, it returns the key ID, generated token, and the TTL in seconds.
func (api *API) generateTokenHandler(w http.ResponseWriter, r *http.Request) {
	var generateTokenRequest GenerateTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&generateTokenRequest); err != nil {
		slog.Error("Failed to decode request", "error", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)

		return
	}

	keyId := generateTokenRequest.KeyID
	if keyId == "" {
		keyId = generateKeyIdForRequest()
	}

	ttl := generateTokenRequest.TTL
	if ttl == 0 {
		ttl = api.config.TokenExpiry
	}

	if ttl == 0 {
		resp := GenerateTokenResponse{
			Success: false,
			Message: "TTL must be greater than 0",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)

		err := json.NewEncoder(w).Encode(resp)
		if err != nil {
			slog.Error("Failed to encode response", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)

			return
		}

		return
	}

	token := token.GenerateToken(keyId).Encode()

	resp := GenerateTokenResponse{
		Success: true,
		Message: "Token generated successfully",
		Token:   token,
		KeyID:   keyId,
		TTL:     ttl,
	}

	w.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		slog.Error("Failed to encode response", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}
	w.WriteHeader(http.StatusOK)
	slog.Info("Token generated successfully", "key_id", keyId, "ttl", ttl)
}

// generateKeyIdForRequest generates a unique key ID for the request.
// It uses a UUID to ensure uniqueness and returns it as a string.
func generateKeyIdForRequest() string {
	return uuid.New().String()
}
