package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"goprl/internal/domain"
)

type mockStore struct {
	mu   sync.RWMutex
	data map[string]*domain.URL
	err  error
}

func (m *mockStore) CreateURL(ctx context.Context, url *domain.URL) error {
	return m.err
}

func (m *mockStore) GetByShortCode(ctx context.Context, code string) (*domain.URL, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.data[code], m.err
}

func (m *mockStore) GetByOriginalURL(ctx context.Context, originalURL string) (*domain.URL, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.data[originalURL], m.err
}

type mockCache struct {
	mu        sync.RWMutex
	data      map[string]*domain.URL
	setCalled bool
	err       error
}

func (m *mockCache) GetURL(ctx context.Context, key string) (*domain.URL, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.data[key], nil
}

func (m *mockCache) SetURL(ctx context.Context, key string, value *domain.URL) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setCalled = true
	return m.err
}

func (m *mockCache) Allow(ctx context.Context, key string, limit int, window time.Duration) error {
	return m.err
}

func (m *mockCache) NextShortCodeID(ctx context.Context) (int64, error) {
	return 1, m.err
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

func TestGenerateShortCode(t *testing.T) {
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
		ShortCode:   "abc",
		OriginalURL: "https://db.com",
		ExpiresAt:   time.Now().Add(-1 * time.Hour),
	}

	store := &mockStore{data: map[string]*domain.URL{"abc": testURL}}
	cache := &mockCache{data: map[string]*domain.URL{"abc": testURL}}
	bloom := &mockBloom{data: map[string]bool{"abc": true}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := New(store, cache, cache, bloom, logger)

	_, err := service.Resolve(ctx, "abc")

	if !errors.Is(err, domain.ErrURLExpired) {
		t.Fatalf("expected error: %v", err)
	}
}

func TestResolve_CacheHit(t *testing.T) {
	ctx := context.Background()

	testURL := &domain.URL{
		ShortCode:   "abc",
		OriginalURL: "https://db.com",
		ExpiresAt:   time.Now().Add(time.Hour),
	}

	store := &mockStore{data: map[string]*domain.URL{"abc": testURL}}
	cache := &mockCache{data: map[string]*domain.URL{"abc": testURL}}
	bloom := &mockBloom{data: map[string]bool{"abc": true}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	service := New(store, cache, cache, bloom, logger)

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
		ShortCode:   "abc",
		OriginalURL: "https://db.com",
		ExpiresAt:   time.Now().Add(time.Hour),
	}

	cache := &mockCache{data: map[string]*domain.URL{}} // Empty cache
	store := &mockStore{data: map[string]*domain.URL{"abc": testURL}}
	bloom := &mockBloom{data: map[string]bool{"abc": true}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := New(store, cache, cache, bloom, logger)

	url, err := svc.Resolve(ctx, "abc")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cache.setCalled {
		t.Error("expected cache fill")
	}

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

	svc := New(store, cache, cache, bloom, logger)

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

	svc := New(store, cache, cache, bloom, logger)

	_, err := svc.Shorten(ctx, testURL.OriginalURL)

	if err == nil {
		t.Fatal("expected error, but got nil")
	}
}
func TestShorten_CacheHit(t *testing.T) {
	ctx := context.Background()

	testURL := &domain.URL{OriginalURL: "https://www.db.com", ShortCode: "abc"}
	store := &mockStore{}
	cache := &mockCache{data: map[string]*domain.URL{"https://www.db.com": testURL}}
	bloom := &mockBloom{data: map[string]bool{"https://www.db.com": true}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := New(store, cache, cache, bloom, logger)

	url, err := svc.Shorten(ctx, "https://www.db.com")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if url.ShortCode != "abc" {
		t.Errorf("expected shortened URL %s, got %s", "abc", url.ShortCode)
	}
}

func TestShorten_CacheMiss_DBHit(t *testing.T) {
	ctx := context.Background()

	testURL := &domain.URL{OriginalURL: "https://www.db.com", ShortCode: "abc"}
	store := &mockStore{data: map[string]*domain.URL{"https://www.db.com": testURL}}
	cache := &mockCache{data: map[string]*domain.URL{}}
	bloom := &mockBloom{data: map[string]bool{"https://www.db.com": true}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := New(store, cache, cache, bloom, logger)

	url, err := svc.Shorten(ctx, "https://www.db.com")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if url.ShortCode != "abc" {
		t.Errorf("expected shortened URL %s, got %s", "abc", url.ShortCode)
	}
}
func TestGenerateBase62(t *testing.T) {
	code := generateBase62(1234)
	if code != "jU" {
		t.Errorf("expected 1234 to be 'jU', got '%s'", code)
	}
}

func TestInvalidURL(t *testing.T) {
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
	svc := New(store, cache, cache, bloom, logger)

	for _, url := range urls {
		_, err := svc.Shorten(ctx, url)
		if err == nil {
			t.Fatalf("expected error for %s, but got nil", url)
		}
	}
}

func TestValidURL(t *testing.T) {
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
	svc := New(store, cache, cache, bloom, logger)

	for _, url := range urls {
		_, err := svc.Shorten(ctx, url)
		if err != nil {
			t.Fatalf("expected no error for %s, but got %v", url, err)
		}
	}
}

type allocatorFunc func(context.Context) (int64, error)

func (f allocatorFunc) NextShortCodeID(ctx context.Context) (int64, error) {
	return f(ctx)
}

type createStore struct {
	mockStore
	create func(context.Context, *domain.URL) error
}

func (s *createStore) CreateURL(ctx context.Context, link *domain.URL) error {
	return s.create(ctx, link)
}

type writeCache struct {
	mockCache
	write func(context.Context, string, *domain.URL) error
}

func (c *writeCache) SetURL(ctx context.Context, key string, link *domain.URL) error {
	return c.write(ctx, key, link)
}

func TestShortenAllocationFailures(t *testing.T) {
	allocationErr := errors.New("sequence unavailable")
	for _, tc := range []struct {
		name  string
		value int64
		err   error
	}{
		{"sequence error", 0, allocationErr},
		{"zero sequence value", 0, nil},
		{"negative sequence value", -1, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &createStore{create: func(context.Context, *domain.URL) error {
				t.Fatal("invalid sequence value must not reach the database")
				return nil
			}}
			allocator := allocatorFunc(func(context.Context) (int64, error) { return tc.value, tc.err })
			svc := New(store, &mockCache{}, allocator, &mockBloom{}, slog.Default())
			link, err := svc.Shorten(context.Background(), "example.com")
			if err == nil || link != nil {
				t.Fatalf("got (%v, %v), want allocation failure", link, err)
			}
			if tc.err != nil && !errors.Is(err, tc.err) {
				t.Fatalf("lost sequence error: %v", err)
			}
		})
	}
}

func TestShortenCollisionReturnsError(t *testing.T) {
	attempts := 0
	allocator := allocatorFunc(func(context.Context) (int64, error) {
		attempts++
		return 1, nil
	})
	store := &createStore{create: func(context.Context, *domain.URL) error {
		return domain.ErrURLAlreadyExists
	}}
	bloom := &mockBloom{data: make(map[string]bool)}
	svc := New(store, nil, allocator, bloom, slog.Default())
	link, err := svc.Shorten(context.Background(), "example.com")
	if link != nil || !errors.Is(err, domain.ErrURLAlreadyExists) || attempts != 1 {
		t.Fatalf("link=%v error=%v allocations=%d", link, err, attempts)
	}
	if bloom.Contains("https://example.com") {
		t.Fatal("failed insert added URL to Bloom filter")
	}
}

func TestShortenPreservesExistingCodes(t *testing.T) {
	original := "https://example.com"
	for _, cacheHit := range []bool{false, true} {
		t.Run(fmt.Sprintf("cacheHit=%v", cacheHit), func(t *testing.T) {
			link := &domain.URL{OriginalURL: original, ShortCode: "abc"}
			store := &mockStore{data: map[string]*domain.URL{original: link}}
			cache := &mockCache{data: make(map[string]*domain.URL)}
			if cacheHit {
				cache.data[original] = link
			}
			allocator := allocatorFunc(func(context.Context) (int64, error) {
				t.Fatal("existing link must not allocate a code")
				return 0, nil
			})
			svc := New(store, cache, allocator, &mockBloom{data: map[string]bool{original: true}}, slog.Default())
			for range 3 {
				got, err := svc.Shorten(context.Background(), original)
				if err != nil || got.ShortCode != "abc" || link.ShortCode != "abc" {
					t.Fatalf("mutated existing link: got=%v original=%v error=%v", got, link, err)
				}
			}
		})
	}
}

func TestCacheWritesAreBestEffortAndBounded(t *testing.T) {
	for _, mode := range []string{"failure", "timeout", "request cancellation"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var logs bytes.Buffer
			writes := 0
			var firstDeadline time.Time
			cache := &writeCache{write: func(ctx context.Context, _ string, link *domain.URL) error {
				writes++
				deadline, ok := ctx.Deadline()
				if !ok || time.Until(deadline) > cacheWriteTimeout {
					t.Fatal("unbounded cache write")
				}
				if writes == 1 {
					firstDeadline = deadline
				} else if deadline != firstDeadline {
					t.Fatal("cache writes do not share a timeout")
				}
				if link.ShortCode != "1" {
					t.Fatalf("cached public URL: %q", link.ShortCode)
				}
				switch mode {
				case "timeout":
					<-ctx.Done()
					return ctx.Err()
				case "request cancellation":
					cancel()
					<-ctx.Done()
					return ctx.Err()
				default:
					return errors.New("cache unavailable")
				}
			}}
			allocator := allocatorFunc(func(context.Context) (int64, error) { return 1, nil })
			svc := New(&mockStore{}, cache, allocator, &mockBloom{data: make(map[string]bool)}, slog.New(slog.NewTextHandler(&logs, nil)))
			link, err := svc.Shorten(ctx, "example.com")
			if err != nil || link == nil || writes != 2 {
				t.Fatalf("link=%v error=%v writes=%d", link, err, writes)
			}
			if !strings.Contains(logs.String(), "cache write failed") {
				t.Fatal("cache failure not logged")
			}
		})
	}
}

func TestServicePreservesDatabaseErrors(t *testing.T) {
	storeErr := errors.New("database unavailable")
	original := "https://example.com"
	svc := New(&mockStore{err: storeErr}, &mockCache{}, nil, &mockBloom{data: map[string]bool{original: true}}, slog.Default())
	if _, err := svc.Shorten(context.Background(), original); !errors.Is(err, storeErr) {
		t.Fatalf("shorten lost error: %v", err)
	}
	if _, err := svc.Resolve(context.Background(), "abc"); !errors.Is(err, storeErr) {
		t.Fatalf("resolve lost error: %v", err)
	}
}

func TestGenerateBase62Boundaries(t *testing.T) {
	for _, n := range []int64{0, 1, 61, 62, 63, 3843, 3844, math.MaxInt64} {
		code := generateBase62(n)
		var decoded int64
		for _, digit := range code {
			decoded = decoded*62 + int64(strings.IndexRune(charset, digit))
		}
		if code == "" || decoded != n {
			t.Errorf("%s encoded to %q, decoded to %d", strconv.FormatInt(n, 10), code, decoded)
		}
	}
}

func TestShortenAndResolveWithoutCache(t *testing.T) {
	data := make(map[string]*domain.URL)
	store := &createStore{mockStore: mockStore{data: data}, create: func(_ context.Context, link *domain.URL) error {
		data[link.ShortCode] = link
		data[link.OriginalURL] = link
		return nil
	}}
	allocations := 0
	allocator := allocatorFunc(func(context.Context) (int64, error) {
		allocations++
		return 62, nil
	})
	svc := New(store, nil, allocator, &mockBloom{data: make(map[string]bool)}, slog.Default())
	link, err := svc.Shorten(context.Background(), "example.com")
	if err != nil || link.ShortCode != "10" {
		t.Fatalf("link=%v error=%v", link, err)
	}
	got, err := svc.Resolve(context.Background(), link.ShortCode)
	if err != nil || got != link {
		t.Fatalf("resolved=%v error=%v", got, err)
	}
	got, err = svc.Shorten(context.Background(), "example.com")
	if err != nil || got != link || allocations != 1 {
		t.Fatalf("reused=%v error=%v allocations=%d", got, err, allocations)
	}
	if _, err := svc.Resolve(context.Background(), "missing"); !errors.Is(err, domain.ErrURLNotFound) {
		t.Fatalf("missing: %v", err)
	}
	link.ExpiresAt = time.Now().Add(-time.Second)
	if _, err := svc.Resolve(context.Background(), link.ShortCode); !errors.Is(err, domain.ErrURLExpired) {
		t.Fatalf("expired: %v", err)
	}
}
