package server

import (
	"context"
	"fmt"
	"time"

	"github.com/ksysoev/make-it-public/pkg/repo/auth"
)

func RunGenerateToken(ctx context.Context, args *args) error {
	if err := initLogger(args); err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}

	cfg, err := loadConfig(args)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	authRepo := auth.New(&cfg.Auth)

	token, err := authRepo.GenerateToken(ctx, time.Hour)
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	fmt.Println("Token:", token.Encode())

	return nil
}
