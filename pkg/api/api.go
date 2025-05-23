// The MIT Server Management API

package api

import (
	"cmp"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"log/slog"

	_ "github.com/ksysoev/make-it-public/pkg/api/docs" // needed for swagger
	"github.com/ksysoev/make-it-public/pkg/api/middleware"
	"github.com/ksysoev/make-it-public/pkg/core/token"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

const (
	DefaultTTLSeconds = int64(3600) // 1 hour
)

type Config struct {
	Listen             string `mapstructure:"listen"`
	DefaultTokenExpiry int64  `mapstructure:"default_token_expiry"`
}

type API struct {
	auth   AuthRepo
	config Config
}

type AuthRepo interface {
	GenerateToken(ctx context.Context, keyID string, ttl time.Duration) (*token.Token, error)
	DeleteToken(ctx context.Context, tokenID string) error
}

const (
	HealthCheckEndpoint   = "GET /health"
	GenerateTokenEndpoint = "POST /generateToken"
	RevokeTokenEndpoint   = "DELETE /token/{keyID}"
	SwaggerEndpoint       = "/swagger/"
)

func New(cfg Config, auth AuthRepo) *API {
	return &API{
		config: cfg,
		auth:   auth,
	}
}

// @title MIT Server Management API
// @version 1.0
// @description This is the API for managing MIT server resources.
// @host localhost:8082
// @BasePath /

// Runs the API management server
func (api *API) Run(ctx context.Context) error {
	router := http.NewServeMux()
	genToken := middleware.Metrics()(http.HandlerFunc(api.generateTokenHandler))
	revokeToken := middleware.Metrics()(http.HandlerFunc(api.RevokeTokenHandler))

	router.Handle(GenerateTokenEndpoint, genToken)
	router.Handle(RevokeTokenEndpoint, revokeToken)
	router.HandleFunc(HealthCheckEndpoint, api.healthCheckHandler)
	router.HandleFunc(SwaggerEndpoint, httpSwagger.WrapHandler)

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
// @Summary Health Check
// @Description Returns the health status of the API.
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 500 {string} string "Internal Server Error"
// @Router /health [get]
func (api *API) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{"status": "healthy"}

	w.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(w).Encode(resp)

	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to encode response", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}
}

// generateTokenHandler is an endpoint to create API token.
// It optionally accepts a key ID, which is automatically generated if not provided.
// It also optionally accepts a TTL for API token, which is set to a default value if not provided.
// As a part of response, it returns the key ID, generated token, and the TTL in seconds
// @Summary Generate Token
// @Description Generates an API token with an optional key ID and TTL.
// @Tags Token
// @Accept json
// @Produce json
// @Param request body GenerateTokenRequest true "Generate Token Request"
// @Success 200 {object} GenerateTokenResponse
// @Failure 400 {string} string "Bad Request"
// @Failure 500 {string} string "Internal Server Error"
// @Router /generateToken [post]
func (api *API) generateTokenHandler(w http.ResponseWriter, r *http.Request) {
	var generateTokenRequest GenerateTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&generateTokenRequest); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)

		return
	}

	keyID := generateTokenRequest.KeyID
	ttl := cmp.Or(generateTokenRequest.TTL, api.config.DefaultTokenExpiry)

	ttl = cmp.Or(ttl, DefaultTTLSeconds)

	generatedToken, err := api.auth.GenerateToken(r.Context(), keyID, time.Second*time.Duration(ttl))
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to generate token", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	resp := GenerateTokenResponse{
		Token: generatedToken.Encode(),
		KeyID: cmp.Or(keyID, generatedToken.ID),
		TTL:   ttl,
	}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to encode response", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusOK)
}

// RevokeTokenHandler revokes an API token by deleting it using the provided Key ID.
// It decodes the incoming JSON request to extract the Key ID and validates it.
// If the Key ID is missing or invalid, it responds with a 400 Bad Request error.
// Returns 204 No Content on successful revocation or 500 Internal Server Error on failure.
// @Summary Revoke Token
// @Description Revokes an API token using the provided Key ID.
// @Tags Token
// @Param keyID path string true "API Key ID"
// @Success 204
// @Failure 400 {string} string "Bad Request"
// @Failure 500 {string} string "Internal Server Error"
// @Router /token/{keyID} [delete]
func (api *API) RevokeTokenHandler(w http.ResponseWriter, r *http.Request) {
	keyID := r.PathValue("keyID")

	if keyID == "" {
		http.Error(w, "Key ID is required", http.StatusBadRequest)
		return
	}

	if err := api.auth.DeleteToken(r.Context(), keyID); err != nil {
		slog.ErrorContext(r.Context(), "Failed to revoke token", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
