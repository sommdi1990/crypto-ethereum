package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Client wraps Redis for cache and distributed locks.
type Client struct {
	rdb *goredis.Client
}

// New connects to Redis.
func New(ctx context.Context, addr string) (*Client, error) {
	rdb := goredis.NewClient(&goredis.Options{Addr: addr})
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &Client{rdb: rdb}, nil
}

// Ping checks connectivity.
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Close closes the connection.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Lock acquires a simple distributed lock with TTL.
func (c *Client) Lock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	ok, err := c.rdb.SetNX(ctx, "lock:"+key, "1", ttl).Result()
	return ok, err
}

// Unlock releases a lock.
func (c *Client) Unlock(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, "lock:"+key).Err()
}
