package token

import (
	"bytes"
	"cmp"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"slices"
	"time"
)

const (
	defaultIDLength     = 8
	maxIDLength         = 15
	defaultSecretLength = 33
	lowerCase           = "abcdefghijklmnopqrstuvwxyz"
	upperCase           = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	numbers             = "0123456789"
	base64Modulo        = 3
	defaultTTLSeconds   = 3600 // 1 hour
)

// TokenType represents the type of token (web or TCP).
type TokenType string

const (
	// TokenTypeWeb represents a token for HTTP/web tunnels.
	TokenTypeWeb TokenType = "w"
	// TokenTypeTCP represents a token for TCP tunnels.
	TokenTypeTCP TokenType = "t"
)

// String returns the user-facing string representation of the token type.
// It maps internal codes to readable names: "w" -> "web", "t" -> "tcp".
func (t TokenType) String() string {
	switch t {
	case TokenTypeWeb:
		return "web"
	case TokenTypeTCP:
		return "tcp"
	default:
		return string(t)
	}
}

type Token struct {
	ID     string
	Secret string // #nosec G117 -- This is a field name, not an exposed secret value
	Type   TokenType
	TTL    time.Duration
}

var (
	ErrTokenTooLong     = fmt.Errorf("token length exceeds maximum limit of %d characters", maxIDLength)
	ErrTokenInvalid     = fmt.Errorf("token contains invalid characters, only lowercase letters and digits are allowed")
	ErrInvalidTokenTTL  = fmt.Errorf("ttl must be positive number")
	ErrInvalidTokenType = fmt.Errorf("token type must be 'w' (web) or 't' (tcp)")
)

// IsValidTokenType checks if the provided token type is valid.
// It returns true if the type is either TokenTypeWeb or TokenTypeTCP.
func IsValidTokenType(t TokenType) bool {
	return t == TokenTypeWeb || t == TokenTypeTCP
}

// GenerateToken creates a new token with the specified keyID, time-to-live (TTL), and token type.
// It validates the keyID's length and characters, generating a random keyID if none is provided.
// Accepts keyID as the identifier for the token, ttl as the duration in seconds, and tokenType as the type of token.
// If ttl is 0, a default value is used. If tokenType is empty, TokenTypeWeb is used.
// Returns the generated Token structure or an error if validation fails, or if ID/secret generation errors occur.
func GenerateToken(keyID string, ttl int, tokenType TokenType) (*Token, error) {
	if len(keyID) > maxIDLength {
		return nil, ErrTokenTooLong
	}

	for _, r := range keyID {
		if !slices.Contains([]rune(lowerCase+numbers), r) {
			return nil, ErrTokenInvalid
		}
	}

	ttl = cmp.Or(ttl, defaultTTLSeconds)

	if ttl <= 0 {
		return nil, ErrInvalidTokenTTL
	}

	// Default to web token type if not specified
	if tokenType == "" {
		tokenType = TokenTypeWeb
	}

	// Validate token type
	if !IsValidTokenType(tokenType) {
		return nil, ErrInvalidTokenType
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
		TTL:    time.Duration(ttl) * time.Second,
		Type:   tokenType,
	}, nil
}

// Encode generates a base64-encoded string representation of the token.
// It combines the token's ID (with type suffix), and Secret, separated by a colon, before encoding.
// The format is: base64(<ID>-<type>:<Secret>) where <type> is 'w' or 't'.
// This format is backward compatible with old clients that expect only ID:Secret.
// Returns the encoded token string.
func (t *Token) Encode() string {
	tokenType := t.Type
	if tokenType == "" {
		tokenType = TokenTypeWeb
	}

	keyWithType := t.ID + "-" + string(tokenType)

	return base64.StdEncoding.EncodeToString([]byte(keyWithType + ":" + t.Secret))
}

// extractTypeFromIDWithValidation extracts the token type from an ID with a type suffix.
// It looks for a pattern like "mykey-w" or "mykey-t" and returns the type and whether a valid suffix was found.
// If no valid type suffix is found, it returns TokenTypeWeb (default) and false.
func extractTypeFromIDWithValidation(id string) (TokenType, bool) {
	lastDash := bytes.LastIndexByte([]byte(id), '-')
	if lastDash == -1 || lastDash == len(id)-1 {
		return TokenTypeWeb, false
	}

	suffix := id[lastDash+1:]
	if suffix == string(TokenTypeWeb) || suffix == string(TokenTypeTCP) {
		return TokenType(suffix), true
	}

	return TokenTypeWeb, false
}

// extractTypeFromID extracts the token type from an ID with a type suffix.
// It looks for a pattern like "mykey-w" or "mykey-t" and returns the type.
// If no valid type suffix is found, it returns TokenTypeWeb as the default.
func extractTypeFromID(id string) TokenType {
	tokenType, _ := extractTypeFromIDWithValidation(id)
	return tokenType
}

// Decode parses a base64-encoded string into a Token instance.
// It validates the encoding and token format, ensuring data integrity.
// Supports two formats:
// 1. New format: base64(<ID>-<type>:<Secret>) where <type> is 'w' or 't'
// 2. Old format: base64(<ID>:<Secret>) defaults to TokenTypeWeb
// Accepts encoded which is a base64-encoded string containing token ID and Secret.
// Returns a Token containing the Type, ID and Secret if decoding is successful.
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

	// Could be new format (<ID>-<type>:<Secret>) or old format (<ID>:<Secret>)
	tokenID := string(parts[0])
	secretPart := string(parts[1])

	// Try to extract type from ID suffix
	tokenType, hasValidSuffix := extractTypeFromIDWithValidation(tokenID)
	if hasValidSuffix {
		// Strip the type suffix from the ID
		if lastDash := bytes.LastIndexByte([]byte(tokenID), '-'); lastDash != -1 {
			tokenID = tokenID[:lastDash]
		}
	}

	if tokenID == "" {
		return nil, fmt.Errorf("invalid token format: empty ID")
	}

	return &Token{
		ID:     tokenID,
		Secret: secretPart,
		Type:   tokenType,
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

// calculateSecretBuffer calculates the required buffer length for a secret based on the given key ID length.
// It ensures that the total length of the ID with type suffix, colon separator, and secret is divisible by base64 encoding factor.
// The format is <ID>-<type>:<secret>, so total length = keyIDLength + 1 (dash) + 1 (type) + 1 (colon) + buffer.
// Returns the calculated buffer length.
func calculateSecretBuffer(keyIDLength int) int {
	buffer := defaultSecretLength

	// Total length = keyIDLength + 1 (dash) + 1 (type char) + 1 (colon) + buffer
	for (keyIDLength+3+buffer)%base64Modulo != 0 {
		buffer++
	}

	return buffer
}
