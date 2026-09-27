package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"goprl/internal/domain"
)

type counterFunc func(context.Context, string) (int64, error)

func (f counterFunc) Increment(ctx context.Context, key string) (int64, error) {
	return f(ctx, key)
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
	counterErr := errors.New("redis unavailable")
	for _, tc := range []struct {
		name  string
		value int64
		err   error
	}{
		{"counter error", 0, counterErr},
		{"zero counter", 0, nil},
		{"negative counter", -1, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &createStore{create: func(context.Context, *domain.URL) error {
				t.Fatal("invalid counter must not reach the database")
				return nil
			}}
			counter := counterFunc(func(context.Context, string) (int64, error) { return tc.value, tc.err })
			svc := New(store, &mockCache{}, counter, &mockBloom{}, slog.Default())
			link, err := svc.Shorten(context.Background(), "example.com")
			if err == nil || link != nil {
				t.Fatalf("got (%v, %v), want allocation failure", link, err)
			}
			if tc.err != nil && !errors.Is(err, tc.err) {
				t.Fatalf("lost counter error: %v", err)
			}
		})
	}
}

func TestShortenCollisionRetry(t *testing.T) {
	for _, exhaust := range []bool{false, true} {
		t.Run(fmt.Sprintf("exhaust=%v", exhaust), func(t *testing.T) {
			attempts := 0
			counter := counterFunc(func(context.Context, string) (int64, error) {
				attempts++
				return int64(attempts), nil
			})
			store := &createStore{create: func(_ context.Context, link *domain.URL) error {
				if link.ShortCode != generateBase62(int64(attempts)) {
					t.Fatalf("retry did not allocate a fresh code: %q", link.ShortCode)
				}
				if exhaust || attempts == 1 {
					return fmt.Errorf("insert: %w", domain.ErrURLAlreadyExists)
				}
				return nil
			}}
			bloom := &mockBloom{data: make(map[string]bool)}
			svc := New(store, &mockCache{}, counter, bloom, slog.Default())
			link, err := svc.Shorten(context.Background(), "example.com")
			if exhaust {
				if !errors.Is(err, domain.ErrURLAlreadyExists) || attempts != maxAllocationAttempts || link != nil {
					t.Fatalf("got link=%v err=%v attempts=%d", link, err, attempts)
				}
				if bloom.Contains("https://example.com") {
					t.Fatal("failed allocation added URL to Bloom filter")
				}
			} else if err != nil || attempts != 2 || link.ShortCode != "2" {
				t.Fatalf("got link=%v err=%v attempts=%d", link, err, attempts)
			}
		})
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
			counter := counterFunc(func(context.Context, string) (int64, error) {
				t.Fatal("existing link must not allocate a code")
				return 0, nil
			})
			svc := New(store, cache, counter, &mockBloom{data: map[string]bool{original: true}}, slog.Default())
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
			counter := counterFunc(func(context.Context, string) (int64, error) { return 1, nil })
			svc := New(&mockStore{}, cache, counter, &mockBloom{data: make(map[string]bool)}, slog.New(slog.NewTextHandler(&logs, nil)))
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
