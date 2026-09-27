package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"goprl/internal/buildinfo"
	"goprl/internal/domain"
	"goprl/internal/service"
)

type apiMockStore struct {
	createURLFunc        func(ctx context.Context, url *domain.URL) error
	getByShortCodeFunc   func(ctx context.Context, code string) (*domain.URL, error)
	getByOriginalURLFunc func(ctx context.Context, originalURL string) (*domain.URL, error)
}

func (m *apiMockStore) CreateURL(ctx context.Context, url *domain.URL) error {
	if m.createURLFunc != nil {
		return m.createURLFunc(ctx, url)
	}
	return nil
}

func (m *apiMockStore) GetByShortCode(ctx context.Context, code string) (*domain.URL, error) {
	if m.getByShortCodeFunc != nil {
		return m.getByShortCodeFunc(ctx, code)
	}
	return nil, nil
}

func (m *apiMockStore) GetByOriginalURL(ctx context.Context, originalURL string) (*domain.URL, error) {
	if m.getByOriginalURLFunc != nil {
		return m.getByOriginalURLFunc(ctx, originalURL)
	}
	return nil, nil
}

type apiMockCache struct{}

func (m *apiMockCache) GetURL(ctx context.Context, key string) (*domain.URL, error) {
	return nil, domain.ErrURLNotFound
}
func (m *apiMockCache) SetURL(ctx context.Context, key string, value *domain.URL) error { return nil }
func (m *apiMockCache) Allow(ctx context.Context, key string, limit int, window time.Duration) error {
	return nil
}
func (m *apiMockCache) Increment(ctx context.Context, key string) (int64, error) { return 1, nil }

type mockPinger struct {
	err error
}

func (m *mockPinger) Ping(ctx context.Context) error {
	return m.err
}

type mockBloom struct {
	data map[string]bool
}

func (m *mockBloom) Add(item string) {
	m.data[item] = true
}

func (m *mockBloom) Contains(item string) bool {
	return m.data[item]
}

var mockBaseURL = "http://test.com"

func TestHandler_HandleHealth(t *testing.T) {
	h := New(nil, &mockPinger{err: errors.New("db down")}, &mockPinger{err: errors.New("redis down")}, mockBaseURL)
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	h.handleHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("expected application/json, got %q", got)
	}
	if got := rr.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("expected no-store, got %q", got)
	}
	var resp healthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected ok status, got %q", resp.Status)
	}
	want := buildinfo.Current()
	if resp.Commit != want.Commit || resp.BuildTime != want.BuildTime || resp.GoVersion != want.GoVersion {
		t.Errorf("unexpected build metadata: %+v", resp.Info)
	}
	if resp.StartedAt != want.StartedAt {
		t.Errorf("expected stable startup time %q, got %q", want.StartedAt, resp.StartedAt)
	}
	if _, err := time.Parse(time.RFC3339, resp.StartedAt); err != nil {
		t.Errorf("invalid startup time: %v", err)
	}
	if resp.UptimeSeconds < 0 || resp.UptimeSeconds > want.UptimeSeconds {
		t.Errorf("unexpected uptime: %d", resp.UptimeSeconds)
	}
}

func TestHandler_HandleReady(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		h := New(nil, &mockPinger{}, &mockPinger{}, mockBaseURL)
		req := httptest.NewRequest("GET", "/ready", nil)
		rr := httptest.NewRecorder()

		h.handleReady(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		if rr.Body.String() != "OK" {
			t.Errorf("expected OK, got %s", rr.Body.String())
		}
	})

	t.Run("PostgresUnavailable", func(t *testing.T) {
		h := New(nil, &mockPinger{err: errors.New("db down")}, &mockPinger{}, mockBaseURL)
		req := httptest.NewRequest("GET", "/ready", nil)
		rr := httptest.NewRecorder()

		h.handleReady(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d", rr.Code)
		}
	})

	t.Run("RedisUnavailable", func(t *testing.T) {
		h := New(nil, &mockPinger{}, &mockPinger{err: errors.New("redis down")}, mockBaseURL)
		req := httptest.NewRequest("GET", "/ready", nil)
		rr := httptest.NewRecorder()

		h.handleReady(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d", rr.Code)
		}
	})
}

func TestHandler_HandleShorten(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("CreateNew", func(t *testing.T) {
		store := &apiMockStore{
			createURLFunc: func(ctx context.Context, url *domain.URL) error {
				url.ID = 1
				url.CreatedAt = time.Now()
				return nil
			},
		}
		svc := service.New(store, &apiMockCache{}, &apiMockCache{}, &mockBloom{data: make(map[string]bool)}, logger)
		h := New(svc, &mockPinger{}, &mockPinger{}, mockBaseURL)

		body := map[string]string{"url": "https://google.com"}
		jsonBody, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/shorten", bytes.NewBuffer(jsonBody))
		rr := httptest.NewRecorder()

		h.handleShorten(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d", rr.Code)
		}

		var resp map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp["short_url"] == "" {
			t.Error("expected short url, got empty")
		}
	})

	t.Run("ExistingURL_BloomHit", func(t *testing.T) {
		existingURL := &domain.URL{
			ID:          1,
			OriginalURL: "https://google.com",
			ShortCode:   "abc",
			ExpiresAt:   time.Now().Add(time.Hour),
		}
		store := &apiMockStore{
			getByOriginalURLFunc: func(ctx context.Context, originalURL string) (*domain.URL, error) {
				return existingURL, nil
			},
		}
		// Bloom hit -> Cache miss -> DB hit
		bloom := &mockBloom{data: map[string]bool{"https://google.com": true}}
		svc := service.New(store, &apiMockCache{}, &apiMockCache{}, bloom, logger)
		h := New(svc, &mockPinger{}, &mockPinger{}, mockBaseURL)

		body := map[string]string{"url": "https://google.com"}
		jsonBody, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/shorten", bytes.NewBuffer(jsonBody))
		rr := httptest.NewRecorder()

		h.handleShorten(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d", rr.Code)
		}

		var resp map[string]string
		json.Unmarshal(rr.Body.Bytes(), &resp)

		expectedFullURL := mockBaseURL + "/abc"
		if resp["short_url"] != expectedFullURL {
			t.Errorf("expected %s, got %s", expectedFullURL, resp["short_url"])
		}
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		svc := service.New(&apiMockStore{}, &apiMockCache{}, &apiMockCache{}, &mockBloom{}, logger)
		h := New(svc, &mockPinger{}, &mockPinger{}, mockBaseURL)

		req := httptest.NewRequest("POST", "/shorten", bytes.NewBufferString("invalid json"))
		rr := httptest.NewRecorder()

		h.handleShorten(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("EmptyURL", func(t *testing.T) {
		svc := service.New(&apiMockStore{}, &apiMockCache{}, &apiMockCache{}, &mockBloom{}, logger)
		h := New(svc, &mockPinger{}, &mockPinger{}, mockBaseURL)

		body := map[string]string{"url": "   "}
		jsonBody, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/shorten", bytes.NewBuffer(jsonBody))
		rr := httptest.NewRecorder()

		h.handleShorten(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("BodyTooLarge", func(t *testing.T) {
		svc := service.New(&apiMockStore{}, &apiMockCache{}, &apiMockCache{}, &mockBloom{}, logger)
		h := New(svc, &mockPinger{}, &mockPinger{}, mockBaseURL)

		oversized := `{"url":"` + string(bytes.Repeat([]byte("a"), maxBytes)) + `"}`
		req := httptest.NewRequest("POST", "/shorten", bytes.NewBufferString(oversized))
		rr := httptest.NewRecorder()

		h.handleShorten(rr, req)

		if rr.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("expected 413, got %d", rr.Code)
		}
	})
}

func TestHandler_HandleResolve(t *testing.T) {
	testURL := &domain.URL{
		OriginalURL: "https://google.com",
		ShortCode:   "abc",
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	store := &apiMockStore{
		getByShortCodeFunc: func(ctx context.Context, code string) (*domain.URL, error) {
			if code == "abc" {
				return testURL, nil
			}
			return nil, domain.ErrURLNotFound
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.New(store, &apiMockCache{}, &apiMockCache{}, &mockBloom{}, logger)
	h := New(svc, &mockPinger{}, &mockPinger{}, mockBaseURL)

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/abc", nil)
		req.SetPathValue("code", "abc")
		rr := httptest.NewRecorder()

		h.handleResolve(rr, req)

		if rr.Code != http.StatusTemporaryRedirect {
			t.Errorf("expected 307, got %d", rr.Code)
		}
		if rr.Header().Get("Location") != "https://google.com" {
			t.Errorf("expected location %s, got %s", "https://google.com", rr.Header().Get("Location"))
		}
	})

	t.Run("Expiry failure", func(t *testing.T) {
		testURL := &domain.URL{
			OriginalURL: "https://google.com",
			ShortCode:   "abc",
			ExpiresAt:   time.Now().Add(-time.Hour),
		}
		store := &apiMockStore{
			getByShortCodeFunc: func(ctx context.Context, code string) (*domain.URL, error) {
				if code == "abc" {
					return testURL, nil
				}
				return nil, domain.ErrURLNotFound
			},
		}
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		svc := service.New(store, &apiMockCache{}, &apiMockCache{}, &mockBloom{}, logger)
		h := New(svc, &mockPinger{}, &mockPinger{}, mockBaseURL)

		req := httptest.NewRequest("GET", "/abc", nil)
		req.SetPathValue("code", "abc")
		rr := httptest.NewRecorder()

		h.handleResolve(rr, req)

		if rr.Code != http.StatusGone {
			t.Errorf("expected 410, got %d", rr.Code)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/xyz", nil)
		req.SetPathValue("code", "xyz")
		rr := httptest.NewRecorder()

		h.handleResolve(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", rr.Code)
		}
	})
}
