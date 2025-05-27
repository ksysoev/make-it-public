package auth

import (
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
	scryptPrefix = "sc:"
	apiKeyPrefix = "API_KEY::"
)

type Config struct {
	RedisAddr string `mapstructure:"redis_addr"`
	Password  string `mapstructure:"redis_password"`
	KeyPrefix string `mapstructure:"key_prefix"`
	Salt      string `mapstructure:"salt"`
}

type Redis interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
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

// Verify checks if the provided secret matches the stored value for the given keyID.
// It retrieves the value from the database using the keyID and keyPrefix.
// Returns true if the secret matches, false if not found or mismatched, and error if a database operation fails.
func (r *Repo) Verify(ctx context.Context, keyID, secret string) (bool, error) {
	secretHash, err := hashSecret(secret, r.salt)
	if err != nil {
		return false, fmt.Errorf("failed to hash secret: %w", err)
	}

	res := r.db.Get(ctx, r.keyPrefix+apiKeyPrefix+keyID)

	switch res.Err() {
	case nil:
		return res.Val() == secretHash, nil
	case redis.Nil:
		return false, nil
	default:
		return false, fmt.Errorf("failed to get key: %w", res.Err())
	}
}

// SaveToken saves a token to the database with a hashed secret and specified TTL.
// It generates a hashed secret using the token's Secret and the Repo's salt.
// Returns an error if hashing fails, or if the database operation encounters an issue.
// Returns core.ErrDuplicateTokenID if a token with the same ID already exists.
func (r *Repo) SaveToken(ctx context.Context, t *token.Token) error {
	secretHash, err := hashSecret(t.Secret, r.salt)
	if err != nil {
		return fmt.Errorf("failed to encrypt secret: %w", err)
	}

	res := r.db.SetNX(ctx, r.keyPrefix+apiKeyPrefix+t.ID, secretHash, t.TTL)

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
