package auth

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	RedisAddr string `mapstructure:"redis_addr"`
	Pass      string `mapstructure:"pass"`
	KeyPrefix string `mapstructure:"key_prefix"`
}

type Redis interface {
	Get(ctx context.Context, key string) *redis.StringCmd
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
		Password: cfg.Pass,
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
