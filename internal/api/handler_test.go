package api

import (
	"bytes"
	"context"
	"encoding/json"
	"goprl/internal/domain"
	"goprl/internal/service"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type apiMockStore struct {
	createURLFunc     func(ctx context.Context, url *domain.URL) error
	getByShortURLFunc func(ctx context.Context, code string) (*domain.URL, error)
}

func (m *apiMockStore) CreateURL(ctx context.Context, url *domain.URL) error {
	if m.createURLFunc != nil {
		return m.createURLFunc(ctx, url)
	}
	return nil
}

func (m *apiMockStore) GetByShortURL(ctx context.Context, code string) (*domain.URL, error) {
	if m.getByShortURLFunc != nil {
		return m.getByShortURLFunc(ctx, code)
	}
	return nil, nil
}

type apiMockCache struct{}

func (m *apiMockCache) Get(ctx context.Context, key string) (*domain.URL, error)     { return nil, nil }
func (m *apiMockCache) Set(ctx context.Context, key string, value *domain.URL) error { return nil }
func (m *apiMockCache) Allow(ctx context.Context, key string, limit int, window time.Duration) error {
	return nil
}
func (m *apiMockCache) Increment(ctx context.Context, key string) (int64, error) { return 0, nil }

var mockBaseURL = "http://test.com"

func TestHandler_HandleHealth(t *testing.T) {
	h := NewHandler(nil)
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	h.handleHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if rr.Body.String() != "OK" {
		t.Errorf("expected OK, got %s", rr.Body.String())
	}
}

func TestHandler_HandleShorten(t *testing.T) {
	store := &apiMockStore{
		createURLFunc: func(ctx context.Context, url *domain.URL) error {
			url.ID = 1
			url.CreatedAt = time.Now()
			return nil
		},
	}
	svc := service.NewURLService(store, &apiMockCache{}, nil, mockBaseURL)
	h := NewHandler(svc)

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

	if resp["expires_at"] == "" {
		t.Errorf("expected expires_at, got empty")
	}
	if resp["short_url"] == "" {
		t.Error("expected short url, got empty")
	}
}

func TestHandler_HandleResolve(t *testing.T) {
	testURL := &domain.URL{OriginalURL: "https://google.com", ShortURL: "abc"}
	store := &apiMockStore{
		getByShortURLFunc: func(ctx context.Context, code string) (*domain.URL, error) {
			if code == "abc" {
				return testURL, nil
			}
			return nil, domain.ErrURLNotFound
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.NewURLService(store, &apiMockCache{}, logger, mockBaseURL)
	h := NewHandler(svc)

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/abc", nil)
		req.SetPathValue("code", "abc")
		rr := httptest.NewRecorder()

		h.handleResolve(rr, req)

		if rr.Code != http.StatusMovedPermanently {
			t.Errorf("expected 301, got %d", rr.Code)
		}
		if rr.Header().Get("Location") != "https://google.com" {
			t.Errorf("expected location %s, got %s", "https://google.com", rr.Header().Get("Location"))
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
