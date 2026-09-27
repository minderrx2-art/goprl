package redis

import (
	"context"
	"encoding/json"
	"time"

	"goprl/internal/domain"

	goredis "github.com/go-redis/redis/v8"
)

type Cache struct {
	rdb *goredis.Client
}

func New(rdb *goredis.Client) *Cache {
	return &Cache{rdb: rdb}
}

func (c *Cache) Close() error {
	return c.rdb.Close()
}

func (c *Cache) GetURL(ctx context.Context, key string) (*domain.URL, error) {
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

func (c *Cache) SetURL(ctx context.Context, key string, value *domain.URL) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	// Cache eviction is independent of the service's link-expiry checks.
	return c.rdb.Set(ctx, key, data, time.Hour).Err()
}

// Increment and expiry run atomically so a new window cannot lose its TTL.
var rateLimitScript = goredis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if count == 1 then
    redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return count
`)

func (c *Cache) Allow(ctx context.Context, key string, limit int, window time.Duration) error {
	n, err := rateLimitScript.Run(ctx, c.rdb, []string{"rate_limit:" + key}, window.Milliseconds()).Int64()
	if err != nil {
		return err
	}
	if n > int64(limit) {
		return domain.ErrRateLimitExceeded
	}
	return nil
}

func (c *Cache) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}
