// Package redisx builds Redis clients shared by cache, pub/sub and asynq.
package redisx

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

// NewClient parses REDIS_URL and verifies connectivity.
func NewClient(ctx context.Context, url string) (*redis.Client, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse REDIS_URL: %w", err)
	}
	c := redis.NewClient(opt)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := c.Ping(pingCtx).Err(); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return c, nil
}

// AsynqOpt converts REDIS_URL into asynq connection options.
func AsynqOpt(url string) (asynq.RedisClientOpt, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return asynq.RedisClientOpt{}, fmt.Errorf("parse REDIS_URL: %w", err)
	}
	return asynq.RedisClientOpt{
		Network:   opt.Network,
		Addr:      opt.Addr,
		Username:  opt.Username,
		Password:  opt.Password,
		DB:        opt.DB,
		TLSConfig: opt.TLSConfig,
	}, nil
}
