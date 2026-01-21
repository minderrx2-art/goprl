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

	store := NewCache(rdb)
	if err := store.Set(ctx, "test", &domain.URL{
		ShortURL:    "abc",
		OriginalURL: "https://google.com",
		CreatedAt:   time.Now(),
		ExpiresAt:   nil,
	}); err != nil {
		t.Fatalf("got unexpected error: %v", err)
	}
	url, err := store.Get(ctx, "test")
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
