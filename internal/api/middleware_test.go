package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"goprl/internal/domain"
)

func TestRequestIDMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := RequestID(r.Context())
		if id == "" {
			t.Error("request ID is empty")
		}
	})

	handler := RequestIDMiddleware(next)
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	id := rr.Header().Get("X-Request-ID")
	if id == "" {
		t.Error("X-Request-ID header not set")
	}
}

func TestLoggingMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	handler := LoggingMiddleware(logger)(next)
	req := httptest.NewRequest("GET", "/test-url", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	output := buf.String()
	if !strings.Contains(output, "method=GET") {
		t.Errorf("expected log to contain 'method=GET', got %s", output)
	}
	if !strings.Contains(output, "url=/test-url") {
		t.Errorf("expected log to contain 'url=/test-url', got %s", output)
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	t.Run("Allowed", func(t *testing.T) {
		cache := &apiMockCache{}
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := RateLimitMiddleware(cache, 20, slog.New(slog.NewTextHandler(io.Discard, nil)))(next)
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "127.0.0.1:1234"
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("RateLimited", func(t *testing.T) {
		mockCache := &mockRateLimitCache{
			allowFunc: func(ctx context.Context, key string, limit int, window time.Duration) error {
				return domain.ErrRateLimitExceeded
			},
		}
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

		handler := RateLimitMiddleware(mockCache, 20, slog.New(slog.NewTextHandler(io.Discard, nil)))(next)
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "127.0.0.1:1234"
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusTooManyRequests {
			t.Errorf("expected 429, got %d", rr.Code)
		}
	})
}

type mockRateLimitCache struct {
	apiMockCache
	allowFunc func(ctx context.Context, key string, limit int, window time.Duration) error
}

func (m *mockRateLimitCache) Allow(ctx context.Context, key string, limit int, window time.Duration) error {
	if m.allowFunc != nil {
		return m.allowFunc(ctx, key, limit, window)
	}
	return nil
}

func TestRateLimitPolicies(t *testing.T) {
	for _, tc := range []struct {
		name       string
		path       string
		addr       string
		limit      int
		limiterErr error
		wantCalls  int
		wantKey    string
		wantLog    bool
	}{
		{"health probe", "/health", "127.0.0.1:80", 20, domain.ErrRateLimitExceeded, 0, "", false},
		{"ready probe", "/ready", "127.0.0.1:80", 20, domain.ErrRateLimitExceeded, 0, "", false},
		{"disabled", "/", "127.0.0.1:80", 0, nil, 0, "", false},
		{"negative limit", "/", "127.0.0.1:80", -1, nil, 0, "", false},
		{"IPv6", "/", "[::1]:80", 20, nil, 1, "::1", false},
		{"bare IP", "/", "127.0.0.1", 20, nil, 1, "127.0.0.1", false},
		{"malformed address", "/", "bad-address", 20, nil, 1, "bad-address", false},
		{"fail open", "/", "127.0.0.1:80", 20, errors.New("redis down"), 1, "127.0.0.1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			limiter := &mockRateLimitCache{allowFunc: func(_ context.Context, key string, limit int, window time.Duration) error {
				calls++
				if key != tc.wantKey || limit != tc.limit || window != time.Minute {
					t.Fatalf("unexpected limiter inputs: %q %d %v", key, limit, window)
				}
				return tc.limiterErr
			}}
			var logs bytes.Buffer
			h := RateLimitMiddleware(limiter, tc.limit, slog.New(slog.NewTextHandler(&logs, nil)))(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.RemoteAddr = tc.addr
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != http.StatusNoContent || calls != tc.wantCalls {
				t.Fatalf("status=%d calls=%d", rr.Code, calls)
			}
			if strings.Contains(logs.String(), "rate limiter unavailable") != tc.wantLog {
				t.Fatalf("unexpected logs: %s", &logs)
			}
		})
	}
}
