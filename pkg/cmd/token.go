package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/ksysoev/make-it-public/pkg/core"
	"github.com/ksysoev/make-it-public/pkg/repo/auth"
)

const (
	secondsInHour = 3600
)

// RunGenerateToken generates a new authentication token with a specified key ID and TTL.
// It initializes necessary services such as the logger and configuration loader,
// validates inputs, and creates the token, printing the details upon success.
// ctx is the context for managing request deadlines and cancellations.
// args are the application configuration parameters.
// keyID is the unique identifier for the token being generated.
// keyTTL specifies the token's time to live in hours; it must be greater than 0.
// Returns an error if any step in initialization, configuration loading, or token generation fails.
func RunGenerateToken(ctx context.Context, args *args, keyID string, keyTTL int) error {
	if keyTTL < 1 {
		return fmt.Errorf("key TTL must be greater than 0")
	}

	if err := initLogger(args); err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}

	cfg, err := loadConfig(args)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	authRepo := auth.New(&cfg.Auth)
	svc := core.New(nil, authRepo)

	token, err := svc.GenerateToken(ctx, keyID, keyTTL*secondsInHour)
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	fmt.Println("Key ID:", token.ID)
	fmt.Println("Token:", token.Encode())
	fmt.Println("Valid until:", time.Now().Add(token.TTL).Format(time.RFC3339))

	return nil
}
