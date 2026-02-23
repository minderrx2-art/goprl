package domain

import (
	"context"
	"errors"
	"time"
)

var ErrURLNotFound = errors.New("URL not found")
var ErrURLExpired = errors.New("URL expired")
var ErrRateLimitExceeded = errors.New("rate limit exceeded")
var ErrInvalidURL = errors.New("invalid URL")
var ErrInvalidScheme = errors.New("invalid host")

type URL struct {
	ID          int64     `json:"id"`
	OriginalURL string    `json:"original_url"`
	ShortURL    string    `json:"short_code"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
}

type URLStore interface {
	CreateURL(ctx context.Context, url *URL) error
	GetByShortURL(ctx context.Context, code string) (*URL, error)
}

type URLCache interface {
	Get(ctx context.Context, key string) (*URL, error)
	Set(ctx context.Context, key string, value *URL) error
	Allow(ctx context.Context, key string, limit int, window time.Duration) error
	Increment(ctx context.Context, key string) (int64, error)
}
