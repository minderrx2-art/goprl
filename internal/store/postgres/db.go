package postgres

import (
	"context"
	"database/sql"
	"errors"
	"goprl/internal/domain"

	"github.com/jackc/pgx/v5/pgconn"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) CreateURL(ctx context.Context, url *domain.URL) error {
	query := `INSERT INTO urls (short_code, original_url, expires_at) VALUES ($1, $2, $3) RETURNING id, created_at`
	row := s.db.QueryRowContext(ctx, query, url.ShortCode, url.OriginalURL, url.ExpiresAt)
	err := row.Scan(&url.ID, &url.CreatedAt)

	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrCollision
		}
		return err
	}

	return nil
}

func (s *Store) GetByShortCode(ctx context.Context, shortCode string) (*domain.URL, error) {
	query := `SELECT id, short_code, original_url, expires_at, created_at FROM urls WHERE short_code = $1`
	row := s.db.QueryRowContext(ctx, query, shortCode)

	var url domain.URL
	err := row.Scan(&url.ID, &url.ShortCode, &url.OriginalURL, &url.ExpiresAt, &url.CreatedAt)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrURLNotFound
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
