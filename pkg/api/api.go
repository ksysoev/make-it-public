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
	Listen             string `mapstructure:"listen"`
	DefaultTokenExpiry int64  `mapstructure:"token_expiry"`
}

type API struct {
	auth   AuthRepo
	config Config
}

type AuthRepo interface {
	GenerateToken(ctx context.Context, keyID string, ttl time.Duration) (*token.Token, error)
}

type Endpoint string

const (
	HealthCheckEndpoint   Endpoint = "GET /health"
	GenerateTokenEndpoint Endpoint = "POST /generateToken"
)

func New(cfg Config, auth AuthRepo) *API {
	return &API{
		config: cfg,
		auth:   auth,
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

	keyID := generateTokenRequest.KeyID
	if keyID == "" {
		keyID = generateKeyIDForRequest()
	}

	ttl := generateTokenRequest.TTL
	if ttl == 0 {
		ttl = api.config.DefaultTokenExpiry
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

	generatedToken, err := api.auth.GenerateToken(r.Context(), keyID, time.Second*time.Duration(ttl))

	if err != nil {
		slog.Error("Failed to generate token", "error", err)

		resp := GenerateTokenResponse{
			Success: false,
			Message: "Failed to generate token",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err = json.NewEncoder(w).Encode(resp)

		if err != nil {
			slog.Error("Failed to encode response", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)

			return
		}

		return
	}

	resp := GenerateTokenResponse{
		Success: true,
		Message: "Token generated successfully",
		Token:   generatedToken.Encode(),
		KeyID:   keyID,
		TTL:     ttl,
	}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		slog.Error("Failed to encode response", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusOK)
	slog.Info("Token generated successfully", "key_id", keyID, "ttl", ttl)
}

// generateKeyIDForRequest generates a unique key ID for the request.
// It uses a UUID to ensure uniqueness and returns it as a string.
func generateKeyIDForRequest() string {
	return uuid.New().String()
}
