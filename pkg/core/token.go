package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/ksysoev/make-it-public/pkg/core/token"
)

const (
	attemptsToGenerateToken = 3
)

var (
	ErrDuplicateTokenID = fmt.Errorf("duplicate token ID")
	ErrTokenNotFound    = fmt.Errorf("token not found")
)

// GenerateToken generates a new token with the given keyID, time-to-live (TTL), and token type.
// It attempts to save the token to the authentication repository, retrying on duplicate token ID errors.
// Accepts ctx which is the context for the request, keyID as the identifier for the token, ttl as the duration in seconds,
// and tokenType as the type of token (web or tcp).
// Returns the generated token and an error if generation or saving fails, or if all retry attempts are exhausted.
func (s *Service) GenerateToken(ctx context.Context, keyID string, ttl int, tokenType token.TokenType) (*token.Token, error) {
	for i := 0; i < attemptsToGenerateToken; i++ {
		t, err := token.GenerateToken(keyID, ttl, tokenType)
		if err != nil {
			return nil, fmt.Errorf("failed to generate token: %w", err)
		}

		err = s.auth.SaveToken(ctx, t)

		switch {
		case err == nil:
			return t, nil
		case errors.Is(err, ErrDuplicateTokenID):
			if keyID != "" {
				return nil, fmt.Errorf("failed to save token: %w", err)
			}

			// Retry generating a new token
			continue
		default:
			return nil, fmt.Errorf("failed to save token: %w", err)
		}
	}

	return nil, fmt.Errorf("failed to generate token after %d attempts", attemptsToGenerateToken)
}

// DeleteToken removes the token identified by tokenID from the system.
// It performs a deletion operation in the underlying authentication repository.
// Returns an error if the token does not exist or the deletion process fails.
func (s *Service) DeleteToken(ctx context.Context, tokenID string) error {
	return s.auth.DeleteToken(ctx, tokenID)
}
