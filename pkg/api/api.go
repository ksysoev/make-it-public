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
}

const (
	HealthCheckEndpoint   = "GET /health"
	GenerateTokenEndpoint = "POST /generateToken"
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

	router.Handle(GenerateTokenEndpoint, genToken)
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
// As a part of response, it returns the key ID, generated token, and the TTL in seconds.

// generateTokenHandler is an endpoint to create API token.
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
