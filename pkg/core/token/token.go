package token

import (
	"bytes"
	"cmp"
	"encoding/base64"
	"fmt"

	"github.com/google/uuid"
)

type Token struct {
	ID     string
	Secret string
}

// GenerateToken creates a new Token instance with a unique ID and a secure Secret.
// It ensures both the ID and Secret are random strings suitable for use in URLs and secure contexts.
// Returns a pointer to the generated Token containing the ID and Secret.
func GenerateToken(keyID string) *Token {
	// TODO: find better way to generate ids and secrets
	// Id should be unique and easy to use in URL
	// Secret should be unique and hard to guess
	// Both should be strings
	return &Token{
		// Use cmp.Or to set a custom key ID if provided; otherwise, generate a new UUID.
		ID:     cmp.Or(keyID, uuid.New().String()),
		Secret: uuid.New().String(),
	}
}

func (t *Token) Encode() string {
	return base64.StdEncoding.EncodeToString([]byte(t.ID + ":" + t.Secret))
}

func Decode(encoded string) (*Token, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	parts := bytes.SplitN(data, []byte(":"), 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid token format")
	}

	return &Token{
		ID:     string(parts[0]),
		Secret: string(parts[1]),
	}, nil
}
