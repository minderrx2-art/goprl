package postgres

import (
	"context"
	"goprl/internal/domain"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestGetByShortURL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock sql: %v", err)
	}
	defer db.Close()

	store := NewStore(db)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"id", "short_code", "original_url", "created_at", "expires_at"}).
		AddRow(1, "abc", "https://google.com", time.Now(), time.Now().Add(24*time.Hour))

	mock.ExpectQuery("SELECT id, short_code, original_url, created_at, expires_at FROM urls WHERE short_code = \\$1").
		WithArgs("abc").
		WillReturnRows(rows)

	url, err := store.GetByShortURL(ctx, "abc")

	if err != nil {
		t.Errorf("got error: %v, want nil", err)
	}
	if url.OriginalURL != "https://google.com" {
		t.Errorf("got %s, want https://google.com", url.OriginalURL)
	}
}

func TestGetByOriginalURL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock sql: %v", err)
	}
	defer db.Close()

	store := NewStore(db)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"id", "short_code", "original_url", "created_at", "expires_at"}).
		AddRow(1, "abc", "https://google.com", time.Now(), time.Now().Add(24*time.Hour))

	mock.ExpectQuery("SELECT id, short_code, original_url, created_at, expires_at FROM urls WHERE original_url = \\$1").
		WithArgs("https://google.com").
		WillReturnRows(rows)

	url, err := store.GetByOriginalURL(ctx, "https://google.com")

	if err != nil {
		t.Errorf("got error: %v, want nil", err)
	}
	if url.ShortURL != "abc" {
		t.Errorf("got %s, want abc", url.ShortURL)
	}
}
func TestCreateURL(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock sql: %v", err)
	}
	defer db.Close()

	store := NewStore(db)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{"id", "created_at"}).
		AddRow(1, time.Now())

	expiry := time.Now().Add(24 * time.Hour)
	mock.ExpectQuery("INSERT INTO urls \\(short_code, original_url, expires_at\\) VALUES \\(\\$1, \\$2, \\$3\\) RETURNING id, created_at").
		WithArgs("abc", "https://google.com", expiry).
		WillReturnRows(rows)

	mockURL := &domain.URL{
		ShortURL:    "abc",
		OriginalURL: "https://google.com",
		CreatedAt:   time.Now(),
		ExpiresAt:   expiry,
	}

	err = store.CreateURL(ctx, mockURL)

	if err != nil {
		t.Errorf("got error: %v, want nil", err)
	}
}
