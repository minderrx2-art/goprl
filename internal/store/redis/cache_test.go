package redis

import (
	"context"
	"goprl/internal/domain"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

func TestCache_SetGet(t *testing.T) {
	ctx := context.Background()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}

	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	expiry := time.Now().Add(24 * time.Hour)
	store := NewCache(rdb)
	if err := store.SetURL(ctx, "test", &domain.URL{
		ShortURL:    "abc",
		OriginalURL: "https://google.com",
		CreatedAt:   time.Now(),
		ExpiresAt:   expiry,
	}); err != nil {
		t.Fatalf("got unexpected error: %v", err)
	}
	url, err := store.GetURL(ctx, "test")
	if err != nil {
		t.Fatalf("got unexpected error: %v", err)
	}
	if url == nil {
		t.Fatalf("got nil, want *domain.URL")
	}
	if url.OriginalURL != "https://google.com" {
		t.Errorf("got %s, want https://google.com", url.OriginalURL)
	}
}

func TestCache_SetCounter_NoExpiry(t *testing.T) {
	ctx := context.Background()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewCache(rdb)

	if err := store.SetCounter(ctx, "counter", 42); err != nil {
		t.Fatalf("SetCounter: %v", err)
	}

	ttl, err := rdb.TTL(ctx, "counter").Result()
	if err != nil {
		t.Fatalf("TTL: %v", err)
	}
	// Redis returns -1 when a key exists with no expiry.
	if ttl != -1 {
		t.Errorf("expected no expiry (TTL=-1), got %v", ttl)
	}
}
