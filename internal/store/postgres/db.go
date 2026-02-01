package postgres

import (
	"context"
	"database/sql"
	"errors"
	"goprl/internal/domain"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) CreateURL(ctx context.Context, url *domain.URL) error {
	query := `INSERT INTO urls (short_code, original_url, expires_at) VALUES ($1, $2, $3) RETURNING id, created_at`
	row := s.db.QueryRowContext(ctx, query, url.ShortURL, url.OriginalURL, url.ExpiresAt)
	err := row.Scan(&url.ID, &url.CreatedAt)

	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrCollision
		}
		return err
	}

	return nil
}

func (s *Store) GetByShortURL(ctx context.Context, ShortURL string) (*domain.URL, error) {
	query := `SELECT id, short_code, original_url, created_at, expires_at FROM urls WHERE short_code = $1`
	row := s.db.QueryRowContext(ctx, query, ShortURL)

	var url domain.URL
	err := row.Scan(&url.ID, &url.ShortURL, &url.OriginalURL, &url.CreatedAt, &url.ExpiresAt)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrURLNotFound
	}

	if url.ExpiresAt.Before(time.Now()) {
		return nil, domain.ErrURLExpired
	}

	return &url, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
