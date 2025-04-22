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

func (r *Repo) Verify(ctx context.Context, keyID, secret string) (bool, error) {
	res := r.db.Get(ctx, r.keyPrefix+keyID)

	if res.Err() != nil {
		return false, fmt.Errorf("failed to get key: %w", res.Err())
	}

	return res.Val() == secret, nil
}
