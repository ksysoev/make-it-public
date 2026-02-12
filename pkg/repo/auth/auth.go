package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/ksysoev/make-it-public/pkg/core"
	"github.com/ksysoev/make-it-public/pkg/core/token"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/scrypt"
)

const (
	scryptPrefix  = "sc:"
	apiKeyPrefix  = "API_KEY::"
	typeSeparator = ":"
)

type Config struct {
	RedisAddr string `mapstructure:"redis_addr"`
	Password  string `mapstructure:"redis_password"` // #nosec G117 -- This is a config field name, not an exposed password
	KeyPrefix string `mapstructure:"key_prefix"`
	Salt      string `mapstructure:"salt"`
}

type Redis interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	Exists(ctx context.Context, keys ...string) *redis.IntCmd
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Ping(ctx context.Context) *redis.StatusCmd
	Close() error
}

type Repo struct {
	db        Redis
	keyPrefix string
	salt      []byte
}

// New creates and initializes a new Repo instance with the provided configuration.
// It sets up a Redis client using the given Redis address, password, and key prefix from the Config struct.
// Returns a pointer to the initialized Repo. Assumes valid Config is provided and may panic on misconfiguration.
func New(cfg *Config) *Repo {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.Password,
	})

	return &Repo{
		db:        rdb,
		keyPrefix: cfg.KeyPrefix,
		salt:      []byte(cfg.Salt),
	}
}

func (r *Repo) CheckHealth(ctx context.Context) error {
	if err := r.db.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return nil
}

// IsKeyExists checks if a key exists in the database using the specified keyID and keyPrefix.
// It returns true if the key exists, false if it does not, and an error if the database operation fails.
func (r *Repo) IsKeyExists(ctx context.Context, keyID string) (bool, error) {
	res := r.db.Exists(ctx, r.keyPrefix+keyID)

	if res.Err() != nil {
		return false, fmt.Errorf("failed to check key existence: %w", res.Err())
	}

	if res.Val() == 0 {
		return false, nil // Key does not exist
	}

	return true, nil // Key exists
}

// Verify checks if the provided secret matches the stored value for the given keyID.
// It retrieves the value from the database using the keyID and keyPrefix.
// The keyID may contain a type suffix (e.g., "mykey-w" or "mykey-t") which is stripped before lookup.
// Returns true if the secret matches, the token type, and error if a database operation fails.
// For backward compatibility, if no type is stored, it defaults to TokenTypeWeb.
func (r *Repo) Verify(ctx context.Context, keyID, secret string) (bool, token.TokenType, error) {
	secretHash, err := hashSecret(secret, r.salt)
	if err != nil {
		return false, "", fmt.Errorf("failed to hash secret: %w", err)
	}

	// Extract type from keyID suffix (e.g., "mykey-w" -> "mykey", "w")
	actualKeyID, keyIDType := extractTypeFromKeyID(keyID)

	res := r.db.Get(ctx, r.keyPrefix+apiKeyPrefix+actualKeyID)

	switch res.Err() {
	case nil:
		storedValue := res.Val()
		// Parse format: sc:<hash>:<type>
		// For backward compatibility: sc:<hash> defaults to web type
		parts := splitStoredValue(storedValue)
		storedHash := parts[0]
		storedTypeStr := parts[1]

		if storedHash != secretHash {
			return false, "", nil
		}

		// Prefer the stored type over the keyID suffix type
		// But if keyID has a type and stored value doesn't have one (old format), use keyID type
		finalType := token.TokenType(storedTypeStr)
		if storedTypeStr == string(token.TokenTypeWeb) && keyIDType != token.TokenTypeWeb {
			// This might be a default from old format, check if keyID had explicit type
			finalType = keyIDType
		}

		return true, finalType, nil
	case redis.Nil:
		return false, "", nil
	default:
		return false, "", fmt.Errorf("failed to get key: %w", res.Err())
	}
}

// SaveToken saves a token to the database with a hashed secret, token type, and specified TTL.
// It generates a hashed secret using the token's Secret and the Repo's salt.
// The stored value format is: sc:<hash>:<type>
// Returns an error if hashing fails, or if the database operation encounters an issue.
// Returns core.ErrDuplicateTokenID if a token with the same ID already exists.
func (r *Repo) SaveToken(ctx context.Context, t *token.Token) error {
	secretHash, err := hashSecret(t.Secret, r.salt)
	if err != nil {
		return fmt.Errorf("failed to encrypt secret: %w", err)
	}

	// Store format: sc:<hash>:<type>
	tokenType := t.Type
	if tokenType == "" {
		tokenType = token.TokenTypeWeb
	}

	storedValue := secretHash + typeSeparator + string(tokenType)

	res := r.db.SetNX(ctx, r.keyPrefix+apiKeyPrefix+t.ID, storedValue, t.TTL)

	if res.Err() != nil {
		return fmt.Errorf("failed to save token: %w", res.Err())
	}

	if !res.Val() {
		return core.ErrDuplicateTokenID
	}

	return nil
}

// DeleteToken removes a token identified by tokenID from the database using the configured key prefix.
// It returns an error if the deletion operation fails.
func (r *Repo) DeleteToken(ctx context.Context, tokenID string) error {
	res := r.db.Del(ctx, r.keyPrefix+apiKeyPrefix+tokenID)

	if res.Err() != nil {
		return fmt.Errorf("failed to delete token: %w", res.Err())
	}

	if res.Val() == 0 {
		return core.ErrTokenNotFound
	}

	return nil
}

// Close releases any resources associated with the Redis connection.
// Returns an error if the connection fails to close.
func (r *Repo) Close() error {
	return r.db.Close()
}

// hashSecret hashes the secret using the scrypt key derivation function with the provided salt and returns the result.
// It prefixes the result with a constant identifier for scrypt-hashed values.
// Returns the hashed secret as a string and an error if the hashing process fails.
func hashSecret(secret string, salt []byte) (string, error) {
	dk, err := scrypt.Key([]byte(secret), salt, 1<<15, 8, 1, 32)
	if err != nil {
		return "", fmt.Errorf("failed to hash secret: %w", err)
	}

	return scryptPrefix + base64.StdEncoding.EncodeToString(dk), nil
}

// splitStoredValue parses the stored value format: sc:<hash>:<type>
// For backward compatibility with old tokens (sc:<hash>), it defaults to TokenTypeWeb.
// Returns a tuple of [hash, type].
func splitStoredValue(storedValue string) [2]string {
	// Expected format: sc:<hash>:<type>
	parts := bytes.SplitN([]byte(storedValue), []byte(typeSeparator), 3)

	if len(parts) >= 3 {
		// New format with type: sc:<hash>:<type>
		hash := string(parts[0]) + typeSeparator + string(parts[1])
		tokenType := string(parts[2])

		return [2]string{hash, tokenType}
	}

	// Old format without type: sc:<hash> - default to web
	return [2]string{storedValue, string(token.TokenTypeWeb)}
}

// extractTypeFromKeyID extracts the token type from a keyID that may contain a type suffix.
// It looks for a pattern like "mykey-w" or "mykey-t" and returns the actual keyID and type.
// If no valid type suffix is found, it returns the original keyID and TokenTypeWeb as default.
// Returns a tuple of [actualKeyID, tokenType].
func extractTypeFromKeyID(keyID string) (string, token.TokenType) {
	// Look for the last dash in the keyID
	lastDash := -1

	for i := len(keyID) - 1; i >= 0; i-- {
		if keyID[i] == '-' {
			lastDash = i
			break
		}
	}

	if lastDash == -1 || lastDash == len(keyID)-1 {
		// No dash found or dash is at the end
		return keyID, token.TokenTypeWeb
	}

	suffix := keyID[lastDash+1:]
	if suffix == string(token.TokenTypeWeb) || suffix == string(token.TokenTypeTCP) {
		return keyID[:lastDash], token.TokenType(suffix)
	}

	// No valid type suffix found
	return keyID, token.TokenTypeWeb
}
