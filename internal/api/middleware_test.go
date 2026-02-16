package api

import (
	"bytes"
	"context"
	"goprl/internal/config"
	"goprl/internal/domain"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRequestIDMiddleware(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := r.Context().Value(RequestIDKey).(string)
		if !ok {
			t.Error("request ID not found in context")
		}
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
	// We link the logger to the buffer 'buf' so any logs written go into 'buf'
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
		mockConfig := &config.Config{
			RateLimit: 20,
		}
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		handler := RateLimitMiddleware(cache, mockConfig)(next)
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "127.0.0.1:1234"
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("RateLimited", func(t *testing.T) {
		mockConfig := &config.Config{
			RateLimit: 20,
		}
		mockCache := &mockRateLimitCache{
			allowFunc: func(ctx context.Context, key string, limit int, window time.Duration) error {
				return domain.ErrRateLimitExceeded
			},
		}
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

		handler := RateLimitMiddleware(mockCache, mockConfig)(next)
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
