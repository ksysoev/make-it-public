// The MIT Server Management API

package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"log/slog"

	_ "github.com/ksysoev/make-it-public/pkg/api/docs" // needed for swagger
	"github.com/ksysoev/make-it-public/pkg/api/middleware"
	"github.com/ksysoev/make-it-public/pkg/core"
	"github.com/ksysoev/make-it-public/pkg/core/token"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

type Config struct {
	Listen string `mapstructure:"listen"`
}

type API struct {
	svc    Service
	config Config
}

type Service interface {
	GenerateToken(ctx context.Context, keyID string, ttl int) (*token.Token, error)
	DeleteToken(ctx context.Context, tokenID string) error
}

const (
	HealthCheckEndpoint   = "GET /health"
	GenerateTokenEndpoint = "POST /token"
	RevokeTokenEndpoint   = "DELETE /token/{keyID}" //nolint:gosec // false positive, no hardcoded credentials
	SwaggerEndpoint       = "/swagger/"
)

// New initializes and returns a new API instance configured with the provided Config and Service.
// Config defines API server settings, and Service provides token management functionalities.
// Returns a pointer to the API instance.
func New(cfg Config, svc Service) *API {
	return &API{
		config: cfg,
		svc:    svc,
	}
}

// @title MIT Server Management API
// @version 1.0
// @description This is the API for managing MIT server resources.
// @host localhost:8082
// @BasePath /

// Run starts the API server and handles incoming HTTP requests.
// It configures the HTTP routes, middleware, and server settings based on the API's configuration.
// Accepts ctx to gracefully shut down the server when context is canceled.
// Returns error if the server fails to start or encounters issues during runtime.
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

// healthCheckHandler handles a basic health check endpoint that returns the status of the service as a JSON response.
// It writes a JSON-encoded "healthy" status to the response and sets the appropriate Content-Type header.
// Returns an HTTP 500 status code if JSON encoding fails, logging the error context for debugging.
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
// @Success 201 {object} GenerateTokenResponse
// @Failure 400 {string} string "Bad Request"
// @Failure 409 {string} string "Duplicate token ID"
// @Failure 500 {string} string "Internal Server Error"
// @Router /token [post]
func (api *API) generateTokenHandler(w http.ResponseWriter, r *http.Request) {
	var req GenerateTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)

		return
	}

	t, err := api.svc.GenerateToken(r.Context(), req.KeyID, req.TTL)

	switch {
	case errors.Is(err, token.ErrTokenInvalid):
		http.Error(w, token.ErrTokenInvalid.Error(), http.StatusBadRequest)
		return
	case errors.Is(err, token.ErrTokenTooLong):
		http.Error(w, token.ErrTokenTooLong.Error(), http.StatusBadRequest)
		return
	case errors.Is(err, core.ErrInvalidTokenTTL):
		http.Error(w, core.ErrInvalidTokenTTL.Error(), http.StatusBadRequest)
		return
	case errors.Is(err, core.ErrDuplicateTokenID):
		http.Error(w, "Duplicate token ID", http.StatusConflict)
		return
	case err != nil:
		slog.ErrorContext(r.Context(), "Failed to generate token", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	resp := GenerateTokenResponse{
		Token: t.Encode(),
		KeyID: t.ID,
		TTL:   req.TTL,
	}

	w.Header().Set("Content-Type", "application/json")

	if err = json.NewEncoder(w).Encode(resp); err != nil {
		slog.ErrorContext(r.Context(), "Failed to encode response", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusCreated)
}

// RevokeTokenHandler revokes an API token based on the provided key ID in the request path.
// It checks the presence of the key ID and returns an HTTP error if missing.
// Deletes the token and returns a no-content response on success or an internal server error if deletion fails.
// @Summary Revoke Token
// @Description Revokes an API token using the provided Key ID.
// @Tags Token
// @Param keyID path string true "API Key ID"
// @Success 204
// @Failure 400 {string} string "Bad Request"
// @Failure 404 {string} string "Token not found"
// @Failure 500 {string} string "Internal Server Error"
// @Router /token/{keyID} [delete]
func (api *API) RevokeTokenHandler(w http.ResponseWriter, r *http.Request) {
	keyID := r.PathValue("keyID")

	if keyID == "" {
		http.Error(w, "Key ID is required", http.StatusBadRequest)
		return
	}

	err := api.svc.DeleteToken(r.Context(), keyID)

	switch {
	case errors.Is(err, core.ErrTokenNotFound):
		http.Error(w, "Token not found", http.StatusNotFound)
		return
	case err != nil:
		slog.ErrorContext(r.Context(), "Failed to revoke token", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}
