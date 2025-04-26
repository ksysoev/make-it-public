package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/ksysoev/make-it-public/pkg/core/token"
	"github.com/redis/go-redis/v9"
)

var (
	ErrFailedToGenerateToken = fmt.Errorf("failed to generate uniq token")
)

type Config struct {
	RedisAddr string `mapstructure:"redis_addr"`
	Password  string `mapstructure:"redis_password"`
	KeyPrefix string `mapstructure:"key_prefix"`
}

type Redis interface {
	Get(ctx context.Context, key string) *redis.StringCmd
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd
	Close() error
}

type Repo struct {
	db        Redis
	keyPrefix string
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
	}
}

// Verify checks if the provided secret matches the stored value for the given keyID.
// It retrieves the value from the database using the keyID and keyPrefix.
// Returns true if the secret matches, false if not found or mismatched, and error if a database operation fails.
func (r *Repo) Verify(ctx context.Context, keyID, secret string) (bool, error) {
	res := r.db.Get(ctx, r.keyPrefix+keyID)

	switch res.Err() {
	case nil:
		return res.Val() == secret, nil
	case redis.Nil:
		return false, nil
	default:
		return false, fmt.Errorf("failed to get key: %w", res.Err())
	}
}

// GenerateToken generates a new token, stores its secret in the database with a specified TTL, and returns the token.
// It attempts up to 3 times to store the token, ensuring the operation completes successfully.
// Returns the generated token on success or an error if all attempts fail due to database issues or conflicts.
func (r *Repo) GenerateToken(ctx context.Context, ttl time.Duration) (*token.Token, error) {
	for i := 0; i < 3; i++ {
		t := token.GenerateToken()

		// TODO: we should store hash of the token instead of the token itself
		res := r.db.SetNX(ctx, r.keyPrefix+t.ID, t.Secret, ttl)

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
