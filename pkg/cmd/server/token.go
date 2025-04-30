package server

import (
	"context"
	"fmt"
	"time"

	"github.com/ksysoev/make-it-public/pkg/repo/auth"
)

// RunGenerateToken initializes the logger, loads configuration, and generates a new token for authentication.
// It takes a context for request scoping, and args containing configuration details like path, log level, and format.
// Returns an error if logger initialization, configuration loading, or token generation fails.
func RunGenerateToken(ctx context.Context, args *args, keyID string) error {
	if err := initLogger(args); err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}

	cfg, err := loadConfig(args)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	authRepo := auth.New(&cfg.Auth)

	token, err := authRepo.GenerateToken(ctx, keyID, time.Hour)
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	fmt.Println("Key ID:", token.ID)
	fmt.Println("Token:", token.Encode())

	return nil
}
