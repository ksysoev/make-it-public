package token

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"slices"
)

const (
	defaultIDLength     = 8
	maxIDLength         = 15
	defaultSecretLength = 33
	lowerCase           = "abcdefghijklmnopqrstuvwxyz"
	upperCase           = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	numbers             = "0123456789"
	base64Modulo        = 3
)

type Token struct {
	ID     string
	Secret string
}

var (
	ErrTokenTooLong = fmt.Errorf("token length exceeds maximum limit of %d characters", maxIDLength)
	ErrTokenInvalid = fmt.Errorf("token contains invalid characters, only lowercase letters and digits are allowed")
)

// GenerateToken creates a new token with the provided keyID.
// It validates the keyID to ensure it only contains lowercase letters and digits and is within length constraints.
// If keyID is empty, a random ID is generated. The secret is generated to ensure base64 encoding compatibility.
// Returns the generated token and an error if keyID is invalid, exceeds the maximum length, or if secret generation fails.
func GenerateToken(keyID string) (*Token, error) {
	if len(keyID) > maxIDLength {
		return nil, ErrTokenTooLong
	}

	for _, r := range keyID {
		if !slices.Contains([]rune(lowerCase+numbers), r) {
			return nil, ErrTokenInvalid
		}
	}

	if keyID == "" {
		id, err := generateID()
		if err != nil {
			return nil, fmt.Errorf("failed to generate token ID: %w", err)
		}

		keyID = id
	}

	bufferLen := calculateSecretBuffer(len(keyID))
	secret, err := generateSecret(bufferLen)

	if err != nil {
		return nil, fmt.Errorf("failed to generate token secret: %w", err)
	}

	return &Token{
		ID:     keyID,
		Secret: secret,
	}, nil
}

// Encode generates a base64-encoded string representation of the token.
// It combines the token's ID and Secret, separated by a colon, before encoding.
// Returns the encoded token string.
func (t *Token) Encode() string {
	return base64.StdEncoding.EncodeToString([]byte(getTokenPair(t.ID, t.Secret)))
}

// Decode parses a base64-encoded string into a Token instance.
// It validates the encoding and token format, ensuring data integrity.
// Accepts encoded which is a base64-encoded string containing token ID and Secret separated by a colon.
// Returns a Token containing the ID and Secret if decoding is successful.
// Returns an error if the base64 string is invalid or the token format is malformed.
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

// generateID creates a random alphanumeric string of defaultIDLength.
// It combines characters from lowerCase and numbers.
// Key id is used in domain names, and domain names are case-insensitive.
// Returns the generated ID or an error if randomIntSlice fails.
func generateID() (string, error) {
	indices, err := randomIntSlice(len(lowerCase+numbers), defaultIDLength)
	if err != nil {
		return "", err
	}

	b := make([]byte, defaultIDLength)

	for i, idx := range indices {
		b[i] = (lowerCase + numbers)[idx]
	}

	return string(b), nil
}

// generateSecret generates a random alphanumeric string of specified length.
// It uses lowercase letters, uppercase letters, and digits to create the secret.
// bufferLen specifies the length of the generated secret.
// Returns the generated secret string and an error if random number generation fails.
func generateSecret(bufferLen int) (string, error) {
	indices, err := randomIntSlice(len(lowerCase+upperCase+numbers), bufferLen)

	if err != nil {
		return "", err
	}

	b := make([]byte, bufferLen)

	for i, idx := range indices {
		b[i] = (lowerCase + upperCase + numbers)[idx]
	}

	return string(b), nil
}

// randomIntSlice generates a slice of random integers with values less than maxLen and of specified length.
// It uses cryptographic randomness as the source and returns an error if random number generation fails.
func randomIntSlice(maxLen, length int) ([]int, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)

	if err != nil {
		return nil, err
	}

	out := make([]int, length)
	for i := 0; i < length; i++ {
		out[i] = int(b[i]) % maxLen
	}

	return out, nil
}

// getTokenPair concatenates the provided ID and secret with a colon separator.
// It generates a string that combines both components, intended for encoding or further processing.
// Returns the concatenated string in the format "ID:Secret".
func getTokenPair(id, secret string) string {
	return id + ":" + secret
}

// calculateSecretBuffer calculates the required buffer length for a secret based on the given key ID length.
// It ensures that the total length of the key ID, buffer, and separator is divisible by a base64 encoding factor.
// Returns the calculated buffer length.
func calculateSecretBuffer(keyIDLength int) int {
	buffer := defaultSecretLength

	for (keyIDLength+buffer+1)%base64Modulo != 0 {
		buffer++
	}

	return buffer
}
