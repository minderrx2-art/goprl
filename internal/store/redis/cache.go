package redis

import (
	"context"
	"encoding/json"
	"goprl/internal/domain"
	"time"

	"github.com/go-redis/redis/v8"
)

type Cache struct {
	rdb *redis.Client
}

func NewCache(rdb *redis.Client) *Cache {
	return &Cache{rdb: rdb}
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
