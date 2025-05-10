package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/ksysoev/make-it-public/pkg/core/token"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/scrypt"
)

const (
	scryptPrefix = "sc:"
)

var (
	ErrFailedToGenerateToken = fmt.Errorf("failed to generate uniq token")
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
	secretHash, err := r.hashSecret(secret)
	if err != nil {
		return false, fmt.Errorf("failed to hash secret: %w", err)
	}

	res := r.db.Get(ctx, r.keyPrefix+keyID)

	switch res.Err() {
	case nil:
		return res.Val() == secretHash, nil
	case redis.Nil:
		return false, nil
	default:
		return false, fmt.Errorf("failed to get key: %w", res.Err())
	}
}

// GenerateToken creates and stores a unique authentication token in the database with a specified time-to-live (TTL).
// It attempts to save the token up to three times in case of collisions and encrypts the token secret before storage.
// Accepts ctx for request scoping, keyID as the identifier for the token, and ttl as the token's lifespan duration.
// Returns the generated token or an error if token generation, encryption, or database storage fails.
func (r *Repo) GenerateToken(ctx context.Context, keyID string, ttl time.Duration) (*token.Token, error) {
	for i := 0; i < 3; i++ {
		t, err := token.GenerateToken(keyID)

		if err != nil {
			return nil, err
		}

		secretHash, err := r.hashSecret(t.Secret)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt secret: %w", err)
		}

		res := r.db.SetNX(ctx, r.keyPrefix+t.ID, secretHash, ttl)

		if res.Err() != nil {
			return nil, fmt.Errorf("failed to save token: %w", res.Err())
		}

		if !res.Val() {
			continue
		}

		return t, nil
	}

	return nil, ErrFailedToGenerateToken
}

// Close releases any resources associated with the Redis connection.
// Returns an error if the connection fails to close.
func (r *Repo) Close() error {
	return r.db.Close()
}

// hashSecret hashes the given secret using the scrypt key derivation function with the Repo's salt.
// It returns the hashed secret as a base64-encoded string or an error if the hashing process fails.
func (r *Repo) hashSecret(secret string) (string, error) {
	dk, err := scrypt.Key([]byte(secret), r.salt, 1<<15, 8, 1, 32)
	if err != nil {
		return "", fmt.Errorf("failed to hash secret: %w", err)
	}

	return scryptPrefix + base64.StdEncoding.EncodeToString(dk), nil
}
