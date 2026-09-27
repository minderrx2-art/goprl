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

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) CreateURL(ctx context.Context, url *domain.URL) error {
	query := `INSERT INTO urls (short_code, original_url, expires_at) VALUES ($1, $2, $3) RETURNING id, created_at`
	row := s.db.QueryRowContext(ctx, query, url.ShortCode, url.OriginalURL, url.ExpiresAt)
	err := row.Scan(&url.ID, &url.CreatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrURLAlreadyExists
		}
		return err
	}

	return nil
}

func (s *Store) GetByShortCode(ctx context.Context, code string) (*domain.URL, error) {
	query := `SELECT id, short_code, original_url, created_at, expires_at FROM urls WHERE short_code = $1`
	row := s.db.QueryRowContext(ctx, query, code)

	var url domain.URL
	var expiresAt sql.NullTime
	err := row.Scan(&url.ID, &url.ShortCode, &url.OriginalURL, &url.CreatedAt, &expiresAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrURLNotFound
	}
	if err != nil {
		return nil, err
	}

	if expiresAt.Valid {
		url.ExpiresAt = expiresAt.Time
	}

	return &url, nil
}

func (s *Store) GetByOriginalURL(ctx context.Context, originalURL string) (*domain.URL, error) {
	query := `SELECT id, short_code, original_url, created_at, expires_at FROM urls WHERE original_url = $1`
	row := s.db.QueryRowContext(ctx, query, originalURL)

	var url domain.URL
	var expiresAt sql.NullTime
	err := row.Scan(&url.ID, &url.ShortCode, &url.OriginalURL, &url.CreatedAt, &expiresAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrURLNotFound
	}
	if err != nil {
		return nil, err
	}
	if expiresAt.Valid {
		url.ExpiresAt = expiresAt.Time
	}

	return &url, nil
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}
