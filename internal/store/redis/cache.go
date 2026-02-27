package redis

import (
	"context"
	"encoding/json"
	"goprl/internal/domain"
	"time"

	goredis "github.com/go-redis/redis/v8"
)

type Cache struct {
	rdb *goredis.Client
}

func NewCache(rdb *goredis.Client) *Cache {
	return &Cache{rdb: rdb}
}

func (c *Cache) Close() error {
	return c.rdb.Close()
}

func (c *Cache) Get(ctx context.Context, key string) (*domain.URL, error) {
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	var url domain.URL
	err = json.Unmarshal([]byte(val), &url)
	if err != nil {
		return nil, err
	}
	return &url, nil
}

func (c *Cache) Set(ctx context.Context, key string, value *domain.URL) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	// tweak expiry later
	return c.rdb.Set(ctx, key, data, time.Hour).Err()
}

// Fixed window rate limiter
func (c *Cache) Allow(ctx context.Context, key string, limit int, window time.Duration) error {
	fullKey := "rate_limit:" + key

	n, err := c.rdb.Incr(ctx, fullKey).Result()
	if err != nil {
		return err
	}

	if n == 1 {
		err = c.rdb.Expire(ctx, fullKey, window).Err()
		if err != nil {
			return err
		}
	}

	if n > int64(limit) {
		return domain.ErrRateLimitExceeded
	}

	return nil
}

func (c *Cache) Increment(ctx context.Context, key string) (int64, error) {
	return c.rdb.Incr(ctx, key).Result()
}

func (c *Cache) SetCounter(ctx context.Context, key string, value int64) error {
	return c.rdb.Set(ctx, key, value, time.Hour).Err()
}
