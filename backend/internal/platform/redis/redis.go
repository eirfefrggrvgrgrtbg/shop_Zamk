package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type Client struct {
	Client *goredis.Client
}

type Option func(*goredis.Client)

// WithHook attaches a Hook to the Redis client.
func WithHook(hook goredis.Hook) Option {
	return func(c *goredis.Client) {
		c.AddHook(hook)
	}
}

func NewClient(ctx context.Context, addr, password string, db int, opts ...Option) (*Client, error) {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	for _, opt := range opts {
		opt(rdb)
	}

	client := &Client{Client: rdb}

	if err := client.Ping(ctx); err != nil {
		rdb.Close()
		return nil, err
	}

	return client, nil
}

func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := c.Client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}
	return nil
}

func (c *Client) Close() error {
	if c.Client != nil {
		return c.Client.Close()
	}
	return nil
}
