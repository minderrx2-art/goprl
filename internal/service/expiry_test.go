package service

import (
	"context"
	"goprl/internal/domain"
	"io"
	"log/slog"
	"testing"
	"time"
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
			expired := &domain.URL{OriginalURL: original, ShortURL: "old", ExpiresAt: time.Now().Add(-time.Hour)}
			store := &mockStore{data: map[string]*domain.URL{original: expired}}
			cache := &mockCache{data: make(map[string]*domain.URL)}
			if cacheHit {
				cache.data[original] = expired
			}
			bloom := &mockBloom{data: map[string]bool{original: true}}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			svc := NewURLService(store, cache, bloom, logger, mockBaseURL)
			before := time.Now()
			link, err := svc.Shorten(context.Background(), original)
			if err != nil {
				t.Fatal(err)
			}
			if link.ShortURL == mockBaseURL+"/old" || link == expired {
				t.Fatal("returned an expired link")
			}
			if link.ExpiresAt.Before(before.Add(24*time.Hour)) || link.ExpiresAt.After(time.Now().Add(24*time.Hour)) {
				t.Errorf("new link must have a 24-hour lifetime, got %v", link.ExpiresAt)
			}
		})
	}
}
