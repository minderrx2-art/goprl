package service

import (
	"context"
	"errors"
	"goprl/internal/domain"
	"io"
	"log/slog"
	"testing"
	"time"
)

type mockStore struct {
	data      map[string]*domain.URL
	err       error
	callCount int
}

func (m *mockStore) CreateURL(ctx context.Context, url *domain.URL) error {
	m.callCount++
	if m.callCount <= 2 {
		return domain.ErrCollision
	}
	return m.err
}

func (m *mockStore) GetByShortURL(ctx context.Context, code string) (*domain.URL, error) {
	return m.data[code], m.err
}

type mockCache struct {
	data      map[string]*domain.URL
	setCalled bool
	err       error
}

func (m *mockCache) Get(ctx context.Context, key string) (*domain.URL, error) {
	return m.data[key], nil
}

func (m *mockCache) Set(ctx context.Context, key string, value *domain.URL) error {
	m.setCalled = true
	return m.err
}

func (m *mockCache) Allow(ctx context.Context, key string, limit int, window time.Duration) error {
	return m.err
}

var mockBaseURL = "http://test.com"

func TestGenerateShortURL(t *testing.T) {
	length := 6

	code := generateShortURL(length)

	if len(code) != length {
		t.Errorf("expected length %d, got %d", length, len(code))
	}

	code2 := generateShortURL(length)
	if code == code2 {
		t.Errorf("expected random codes, but got two identical: %s", code)
	}

	for _, char := range code {
		found := false
		for _, valid := range charset {
			if char == valid {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("code contains invalid character: %c", char)
		}
	}
}

func TestResolve_CacheHit(t *testing.T) {
	ctx := context.Background()

	testURL := &domain.URL{ShortURL: "abc", OriginalURL: "https://db.com"}

	store := &mockStore{data: map[string]*domain.URL{"abc": testURL}}
	cache := &mockCache{data: map[string]*domain.URL{"abc": testURL}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := NewURLService(store, cache, logger, mockBaseURL)

	url, err := service.Resolve(ctx, "abc")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if url.OriginalURL != "https://db.com" {
		t.Errorf("expected url to be https://db.com, got %s", testURL.OriginalURL)
	}

	if cache.setCalled {
		t.Errorf("expected cache.set to be called, but it wasn't")
	}
}

func TestResolve_CacheMiss_DBHit(t *testing.T) {
	ctx := context.Background()

	testURL := &domain.URL{ShortURL: "abc", OriginalURL: "https://db.com"}
	cache := &mockCache{data: map[string]*domain.URL{}} // Empty cache
	store := &mockStore{data: map[string]*domain.URL{"abc": testURL}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := NewURLService(store, cache, logger, mockBaseURL)

	url, err := svc.Resolve(ctx, "abc")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cache.setCalled {
		t.Errorf("expected cache.Set to be called (backfill), but it wasn't")
	}

	if url.OriginalURL != "https://db.com" {
		t.Errorf("got %s, want https://db.com", url.OriginalURL)
	}
}

func TestShorten_DBError(t *testing.T) {
	ctx := context.Background()

	testURL := &domain.URL{OriginalURL: "https://db.com"}
	store := &mockStore{err: errors.New("db error")}
	cache := &mockCache{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := NewURLService(store, cache, logger, mockBaseURL)

	_, err := svc.Shorten(ctx, testURL.OriginalURL)

	if err == nil {
		t.Fatal("expected error, but got nil")
	}
}

func TestShorten_Collision(t *testing.T) {
	ctx := context.Background()

	testURL := &domain.URL{OriginalURL: "https://db.com"}
	store := &mockStore{}
	cache := &mockCache{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := NewURLService(store, cache, logger, mockBaseURL)

	_, err := svc.Shorten(ctx, testURL.OriginalURL)

	if err != nil {
		t.Errorf("Expected success after 2 retries, got %v", err)
	}

	if store.callCount != 3 {
		t.Errorf("Expected 3 calls total, got %d", store.callCount)
	}

}
