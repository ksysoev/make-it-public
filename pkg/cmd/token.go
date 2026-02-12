package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/ksysoev/make-it-public/pkg/core"
	"github.com/ksysoev/make-it-public/pkg/core/token"
	"github.com/ksysoev/make-it-public/pkg/repo/auth"
)

const (
	secondsInHour = 3600
)

// RunGenerateToken generates a new authentication token with a specified key ID, TTL, and type.
// It initializes necessary services such as the logger and configuration loader,
// validates inputs, and creates the token, printing the details upon success.
// ctx is the context for managing request deadlines and cancellations.
// args are the application configuration parameters.
// keyID is the unique identifier for the token being generated.
// keyTTL specifies the token's time to live in hours; it must be greater than 0.
// tokenTypeStr specifies the token type: "web" or "tcp".
// Returns an error if any step in initialization, configuration loading, or token generation fails.
func RunGenerateToken(ctx context.Context, args *args, keyID string, keyTTL int, tokenTypeStr string) error {
	if keyTTL < 1 {
		return fmt.Errorf("key TTL must be greater than 0")
	}

	// Validate and map token type
	var tokenType token.TokenType

	switch tokenTypeStr {
	case "web":
		tokenType = token.TokenTypeWeb
	case "tcp":
		tokenType = token.TokenTypeTCP
	default:
		return fmt.Errorf("invalid token type: must be 'web' or 'tcp'")
	}

	if err := initLogger(args); err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}

	cfg, err := loadConfig(args)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	authRepo := auth.New(&cfg.Auth)
	// Pass nil for connection managers since token generation doesn't need them
	svc := core.New(nil, nil, authRepo)

	tok, err := svc.GenerateToken(ctx, keyID, keyTTL*secondsInHour, tokenType)
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	fmt.Println("Key ID:", tok.ID)
	fmt.Println("Token:", tok.Encode())
	fmt.Println("Type:", tok.Type.String())
	fmt.Println("Valid until:", time.Now().Add(tok.TTL).Format(time.RFC3339))

	return nil
}
