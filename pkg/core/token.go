package core

import (
	"cmp"
	"context"
	"time"

	"github.com/ksysoev/make-it-public/pkg/core/token"
)

const (
	defaultTTLSeconds = 3600 // 1 hour
)

// GenerateToken creates a new token for the specified keyID with a given time-to-live (ttl).
// It uses the default TTL if the provided ttl is zero or invalid.
// Returns a pointer to the generated token and an error if token creation fails.
func (s *Service) GenerateToken(ctx context.Context, keyID string, ttl int) (*token.Token, error) {
	ttl = cmp.Or(ttl, defaultTTLSeconds)

	return s.auth.GenerateToken(ctx, keyID, time.Duration(ttl)*time.Second)
}

// DeleteToken removes the token identified by tokenID from the system.
// It performs a deletion operation in the underlying authentication repository.
// Returns an error if the token does not exist or the deletion process fails.
func (s *Service) DeleteToken(ctx context.Context, tokenID string) error {
	return s.auth.DeleteToken(ctx, tokenID)
}
