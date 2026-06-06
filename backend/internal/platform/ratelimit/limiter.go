package ratelimit

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type RedisClient interface {
	Incr(ctx context.Context, key string) *goredis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *goredis.BoolCmd
	TTL(ctx context.Context, key string) *goredis.DurationCmd
}

type Result struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
}

type Limiter struct {
	redis RedisClient
}

func New(redis RedisClient) *Limiter {
	return &Limiter{
		redis: redis,
	}
}

func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (Result, error) {
	if limit <= 0 || window <= 0 {
		return Result{Allowed: true, Remaining: limit}, nil
	}

	count, err := l.redis.Incr(ctx, key).Result()
	if err != nil {
		return Result{}, err
	}

	if count == 1 {
		if err := l.redis.Expire(ctx, key, window).Err(); err != nil {
			return Result{}, err
		}
	}

	remaining := limit - int(count)
	if remaining < 0 {
		remaining = 0
	}

	if count > int64(limit) {
		retryAfter := window
		if ttl, err := l.redis.TTL(ctx, key).Result(); err == nil && ttl > 0 {
			retryAfter = ttl
		}
		return Result{
			Allowed:    false,
			Remaining:  remaining,
			RetryAfter: retryAfter,
		}, ErrLimited
	}

	return Result{
		Allowed:   true,
		Remaining: remaining,
	}, nil
}
