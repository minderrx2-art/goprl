package service

import (
	"context"
	"errors"
	"goprl/internal/domain"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

type mockStore struct {
	mu   sync.RWMutex
	data map[string]*domain.URL
	err  error
}

func (m *mockStore) CreateURL(ctx context.Context, url *domain.URL) error {
	return m.err
}

func (m *mockStore) GetByShortURL(ctx context.Context, code string) (*domain.URL, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.data[code], m.err
}

func (m *mockStore) GetByOriginalURL(ctx context.Context, originalURL string) (*domain.URL, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.data[originalURL], m.err
}

func (m *mockStore) GetMaxID(ctx context.Context) (int64, error) {
	return 0, m.err
}

type mockCache struct {
	mu        sync.RWMutex
	data      map[string]*domain.URL
	setCalled bool
	err       error
}

func (m *mockCache) Get(ctx context.Context, key string) (*domain.URL, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.data[key], nil
}

func (m *mockCache) Set(ctx context.Context, key string, value *domain.URL) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setCalled = true
	return m.err
}

func (m *mockCache) Allow(ctx context.Context, key string, limit int, window time.Duration) error {
	return m.err
}

func (m *mockCache) Increment(ctx context.Context, key string) (int64, error) {
	return 0, m.err
}

func (m *mockCache) SetCounter(ctx context.Context, key string, value int64) error {
	return m.err
}

type mockBloom struct {
	mu   sync.RWMutex
	data map[string]bool
	err  error
}

func (m *mockBloom) Add(item string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[item] = true
}

func (m *mockBloom) Contains(item string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.data[item]
}

var mockBaseURL = "http://test.com"

func TestGenerateShortURL(t *testing.T) {
	code := generateBase62(1)

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

func TestResolveExpiry(t *testing.T) {
	ctx := context.Background()

	testURL := &domain.URL{
		ShortURL:    "abc",
		OriginalURL: "https://db.com",
		ExpiresAt:   time.Now().Add(-1 * time.Hour),
	}

	store := &mockStore{data: map[string]*domain.URL{"abc": testURL}}
	cache := &mockCache{data: map[string]*domain.URL{"abc": testURL}}
	bloom := &mockBloom{data: map[string]bool{"abc": true}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := NewURLService(store, cache, bloom, logger, mockBaseURL)

	_, err := service.Resolve(ctx, "abc")

	if err != domain.ErrURLExpired {
		t.Fatalf("expected error: %v", err)
	}
}

func TestResolve_CacheHit(t *testing.T) {
	ctx := context.Background()

	testURL := &domain.URL{
		ShortURL:    "abc",
		OriginalURL: "https://db.com",
		ExpiresAt:   time.Now().Add(time.Hour),
	}

	store := &mockStore{data: map[string]*domain.URL{"abc": testURL}}
	cache := &mockCache{data: map[string]*domain.URL{"abc": testURL}}
	bloom := &mockBloom{data: map[string]bool{"abc": true}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := NewURLService(store, cache, bloom, logger, mockBaseURL)

	url, err := service.Resolve(ctx, "abc")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if url.OriginalURL != "https://db.com" {
		t.Errorf("expected url to be https://db.com, got %s", testURL.OriginalURL)
	}

	if cache.setCalled {
		t.Errorf("expected cache.set NOT to be called, but it was")
	}
}

func TestResolve_CacheMiss_DBHit(t *testing.T) {
	ctx := context.Background()

	testURL := &domain.URL{
		ShortURL:    "abc",
		OriginalURL: "https://db.com",
		ExpiresAt:   time.Now().Add(time.Hour),
	}

	cache := &mockCache{data: map[string]*domain.URL{}} // Empty cache
	store := &mockStore{data: map[string]*domain.URL{"abc": testURL}}
	bloom := &mockBloom{data: map[string]bool{"abc": true}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := NewURLService(store, cache, bloom, logger, mockBaseURL)

	url, err := svc.Resolve(ctx, "abc")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEventually(t, func() bool {
		cache.mu.RLock()
		defer cache.mu.RUnlock()
		return cache.setCalled
	}, 100*time.Millisecond)

	if url.OriginalURL != "https://db.com" {
		t.Errorf("got %s, want https://db.com", url.OriginalURL)
	}
}

func TestShorten_BloomContains(t *testing.T) {
	ctx := context.Background()

	store := &mockStore{}
	cache := &mockCache{}
	bloom := &mockBloom{data: map[string]bool{"https://www.db.com": true}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := NewURLService(store, cache, bloom, logger, mockBaseURL)

	_, err := svc.Shorten(ctx, "https://db.com")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShorten_DBError(t *testing.T) {
	ctx := context.Background()

	testURL := &domain.URL{OriginalURL: "https://db.com"}
	store := &mockStore{err: errors.New("db error")}
	cache := &mockCache{}
	bloom := &mockBloom{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := NewURLService(store, cache, bloom, logger, mockBaseURL)

	_, err := svc.Shorten(ctx, testURL.OriginalURL)

	if err == nil {
		t.Fatal("expected error, but got nil")
	}
}
func TestShorten_CacheHit(t *testing.T) {
	ctx := context.Background()

	testURL := &domain.URL{OriginalURL: "https://www.db.com", ShortURL: "abc"}
	store := &mockStore{}
	cache := &mockCache{data: map[string]*domain.URL{"https://www.db.com": testURL}}
	bloom := &mockBloom{data: map[string]bool{"https://www.db.com": true}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := NewURLService(store, cache, bloom, logger, mockBaseURL)

	url, err := svc.Shorten(ctx, "https://www.db.com")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if url.ShortURL != mockBaseURL+"/abc" {
		t.Errorf("expected shortened URL %s, got %s", mockBaseURL+"/abc", url.ShortURL)
	}
}

func TestShorten_CacheMiss_DBHit(t *testing.T) {
	ctx := context.Background()

	testURL := &domain.URL{OriginalURL: "https://www.db.com", ShortURL: "abc"}
	store := &mockStore{data: map[string]*domain.URL{"https://www.db.com": testURL}}
	cache := &mockCache{data: map[string]*domain.URL{}}
	bloom := &mockBloom{data: map[string]bool{"https://www.db.com": true}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := NewURLService(store, cache, bloom, logger, mockBaseURL)

	url, err := svc.Shorten(ctx, "https://www.db.com")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if url.ShortURL != mockBaseURL+"/abc" {
		t.Errorf("expected shortened URL %s, got %s", mockBaseURL+"/abc", url.ShortURL)
	}
}
func TestGenerateBase62(t *testing.T) {
	code := generateBase62(1234)
	if code != "jU" {
		t.Errorf("expected 1234 to be 'jU', got '%s'", code)
	}
}

func TestInvalidUrl(t *testing.T) {
	urls := []string{
		"invalid-url",
		"",
		"http://localhost",
		"http://localhost:8080",
		"http://localhost:8080/",
		"http://localhost:8080/",
	}
	ctx := context.Background()
	store := &mockStore{}
	cache := &mockCache{}
	bloom := &mockBloom{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewURLService(store, cache, bloom, logger, mockBaseURL)

	for _, url := range urls {
		_, err := svc.Shorten(ctx, url)
		if err == nil {
			t.Fatalf("expected error for %s, but got nil", url)
		}
	}
}

func TestValidUrl(t *testing.T) {
	urls := []string{
		"http://www.google.com",
		"google.com",
		"www.google.com",
		"https://www.google.com",
		"https://google.com",
	}
	ctx := context.Background()
	store := &mockStore{}
	cache := &mockCache{}
	bloom := &mockBloom{data: make(map[string]bool)}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewURLService(store, cache, bloom, logger, mockBaseURL)

	for _, url := range urls {
		_, err := svc.Shorten(ctx, url)
		if err != nil {
			t.Fatalf("expected no error for %s, but got %v", url, err)
		}
	}
}

func assertEventually(t *testing.T, condition func() bool, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Errorf("condition not met within %v", timeout)
}
