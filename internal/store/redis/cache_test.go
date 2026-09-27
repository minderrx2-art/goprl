package redis

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"goprl/internal/domain"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
)

func TestCache_SetGet(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	t.Cleanup(func() { rdb.Close() })
	expiry := time.Now().Add(24 * time.Hour)
	store := New(rdb)
	if err := store.SetURL(ctx, "test", &domain.URL{
		ShortCode:   "abc",
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

func TestAllowConcurrentWindow(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	cache := New(client)
	const limit = 10
	results := make(chan error, 50)
	for range cap(results) {
		go func() { results <- cache.Allow(context.Background(), "client", limit, time.Minute) }()
	}
	allowed := 0
	for range cap(results) {
		err := <-results
		if err == nil {
			allowed++
		} else if !errors.Is(err, domain.ErrRateLimitExceeded) {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if allowed != limit {
		t.Fatalf("allowed %d requests, want %d", allowed, limit)
	}
	if ttl := mr.TTL("rate_limit:client"); ttl != time.Minute {
		t.Fatalf("TTL=%v, want one minute", ttl)
	}
	mr.FastForward(30 * time.Second)
	if err := cache.Allow(context.Background(), "client", limit, time.Minute); !errors.Is(err, domain.ErrRateLimitExceeded) {
		t.Fatalf("got %v, want rate limit error", err)
	}
	if ttl := mr.TTL("rate_limit:client"); ttl != 30*time.Second {
		t.Fatalf("request extended the fixed window: %v", ttl)
	}
	mr.FastForward(30 * time.Second)
	if err := cache.Allow(context.Background(), "client", limit, time.Minute); err != nil {
		t.Fatalf("new window: %v", err)
	}
}

func TestCachedJSONCompatibility(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	mr.Set("abc", `{"id":1,"short_code":"abc","original_url":"https://example.com"}`)
	cache := New(client)
	link, err := cache.GetURL(context.Background(), "abc")
	if err != nil || link.ShortCode != "abc" {
		t.Fatalf("legacy cache: link=%v err=%v", link, err)
	}
	if err := cache.SetURL(context.Background(), "abc", link); err != nil {
		t.Fatal(err)
	}
	value, err := mr.Get("abc")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(value, `"short_code":"abc"`) || strings.Contains(value, "ShortCode") {
		t.Fatalf("changed cache format: %s", value)
	}
	if ttl := mr.TTL("abc"); ttl != time.Hour {
		t.Fatalf("changed TTL: %v", ttl)
	}
}
