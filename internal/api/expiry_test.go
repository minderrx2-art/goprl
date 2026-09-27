package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"goprl/internal/domain"
	"goprl/internal/service"
	"goprl/internal/store/postgres"
	rediscache "goprl/internal/store/redis"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	goredis "github.com/go-redis/redis/v8"
)

func TestResolveExpiryAcrossStoragePaths(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name       string
		cacheState string
		expiresAt  time.Time
		storeErr   error
		wantStatus int
	}{
		{"expired cache hit", "hit", now.Add(-time.Hour), nil, http.StatusGone},
		{"expired cache miss", "miss", now.Add(-time.Hour), nil, http.StatusGone},
		{"expired after cache eviction", "evicted", now.Add(-time.Hour), nil, http.StatusGone},
		{"active cache hit", "hit", now.Add(time.Hour), nil, http.StatusTemporaryRedirect},
		{"active cache miss", "miss", now.Add(time.Hour), nil, http.StatusTemporaryRedirect},
		{"no expiry cache hit", "hit", time.Time{}, nil, http.StatusTemporaryRedirect},
		{"NULL database expiry", "miss", time.Time{}, nil, http.StatusTemporaryRedirect},
		{"missing link", "miss", time.Time{}, sql.ErrNoRows, http.StatusNotFound},
		{"database failure", "miss", time.Time{}, errors.New("database unavailable"), http.StatusInternalServerError},
		{"wrapped expiry error", "miss", time.Time{}, fmt.Errorf("lookup: %w", domain.ErrURLExpired), http.StatusGone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { db.Close() })
			mr := miniredis.RunT(t)
			rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
			t.Cleanup(func() { rdb.Close() })
			cache := rediscache.New(rdb)
			link := &domain.URL{ID: 1, ShortCode: "abc", OriginalURL: "https://example.com", CreatedAt: now.Add(-24 * time.Hour), ExpiresAt: tc.expiresAt}
			if tc.cacheState != "miss" {
				if err := cache.SetURL(context.Background(), "abc", link); err != nil {
					t.Fatal(err)
				}
				if tc.cacheState == "evicted" {
					mr.FastForward(2 * time.Hour)
				}
			}
			if tc.cacheState != "hit" {
				query := mock.ExpectQuery("SELECT id, short_code, original_url, created_at, expires_at FROM urls WHERE short_code = \\$1").WithArgs("abc")
				if tc.storeErr != nil {
					query.WillReturnError(tc.storeErr)
				} else {
					var expiry any
					if !tc.expiresAt.IsZero() {
						expiry = tc.expiresAt
					}
					query.WillReturnRows(sqlmock.NewRows([]string{"id", "short_code", "original_url", "created_at", "expires_at"}).AddRow(link.ID, link.ShortCode, link.OriginalURL, link.CreatedAt, expiry))
				}
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			svc := service.New(postgres.New(db), cache, postgres.New(db), nil, logger)
			h := New(svc, nil, nil, mockBaseURL)
			req := httptest.NewRequest(http.MethodGet, "/abc", nil)
			req.SetPathValue("code", "abc")
			rr := httptest.NewRecorder()
			h.handleResolve(rr, req)
			if rr.Code != tc.wantStatus {
				t.Fatalf("got %d (%s), want %d", rr.Code, rr.Body.String(), tc.wantStatus)
			}
			if rr.Header().Get("Cache-Control") != "no-store" {
				t.Error("resolution responses must prevent caching past expiry")
			}
			if tc.wantStatus == http.StatusTemporaryRedirect && rr.Header().Get("Location") != link.OriginalURL {
				t.Errorf("unexpected destination: %q", rr.Header().Get("Location"))
			}
			if tc.wantStatus == http.StatusGone && rr.Header().Get("Location") != "" {
				t.Error("expired links must not redirect")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
