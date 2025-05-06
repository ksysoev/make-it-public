package token

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

const (
	idLength     = 8
	secretLength = 31
	alphabet     = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	numbers      = "0123456789"
)

type Token struct {
	ID     string
	Secret string
}

// GenerateToken creates a new Token instance with a unique ID and a secure Secret.
// It ensures both the ID and Secret are random strings suitable for use in URLs and secure contexts.
// Returns a pointer to the generated Token containing the ID and Secret.
func GenerateToken(keyID string) (*Token, error) {
	if keyID == "" {
		id, err := generateID()
		if err != nil {
			return nil, fmt.Errorf("failed to generate token ID: %w", err)
		}
		keyID = id
	}

	secret, err := generateSecret()

	if err != nil {
		return nil, fmt.Errorf("failed to generate token secret: %w", err)
	}

	return &Token{
		ID:     keyID,
		Secret: secret,
	}, nil
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

// TODO: send and cross-verify the ID in to redis and check for duplicates
func generateID() (string, error) {
	b := make([]byte, idLength)

	for i := range b {
		val, err := randomInt(len(alphabet))
		if err != nil {
			return "", err
		}

		b[i] = alphabet[val]
	}

	return string(b), nil
}

func generateSecret() (string, error) {
	b := make([]byte, secretLength)

	for i := range b {
		val, err := randomInt(len(alphabet + numbers))
		if err != nil {
			return "", err
		}

		b[i] = (alphabet + numbers)[val]
	}

	return string(b), nil
}

func randomInt(max int) (int, error) {
	var b [1]byte

	_, err := rand.Read(b[:])
	if err != nil {
		return 0, err
	}

	return int(b[0]) % max, nil
}
