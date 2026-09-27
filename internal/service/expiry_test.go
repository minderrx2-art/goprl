package service

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"goprl/internal/domain"
)

func TestExpiryBoundary(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name   string
		expiry time.Time
		want   bool
	}{
		{"no expiry", time.Time{}, false},
		{"before deadline", now.Add(time.Nanosecond), false},
		{"at deadline", now, true},
		{"past deadline", now.Add(-time.Nanosecond), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isExpired(&domain.URL{ExpiresAt: tc.expiry}, now); got != tc.want {
				t.Errorf("got expired=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestShortenDoesNotReuseExpiredLink(t *testing.T) {
	for _, cacheHit := range []bool{true, false} {
		name := "database hit"
		if cacheHit {
			name = "cache hit"
		}
		t.Run(name, func(t *testing.T) {
			original := "https://example.com"
			expired := &domain.URL{OriginalURL: original, ShortCode: "old", ExpiresAt: time.Now().Add(-time.Hour)}
			store := &mockStore{data: map[string]*domain.URL{original: expired}}
			cache := &mockCache{data: make(map[string]*domain.URL)}
			if cacheHit {
				cache.data[original] = expired
			}
			bloom := &mockBloom{data: map[string]bool{original: true}}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			svc := New(store, cache, cache, bloom, logger)
			before := time.Now()
			link, err := svc.Shorten(context.Background(), original)
			if err != nil {
				t.Fatal(err)
			}
			if link.ShortCode == "old" || link == expired {
				t.Fatal("returned an expired link")
			}
			if link.ExpiresAt.Before(before.Add(linkLifetime)) || link.ExpiresAt.After(time.Now().Add(linkLifetime)) {
				t.Errorf("new link must have a seven-day lifetime, got %v", link.ExpiresAt)
			}
		})
	}
}
