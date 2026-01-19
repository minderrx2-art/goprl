package domain

import (
	"context"
	"time"
)

type URL struct {
	ID          int64     `json:"id"`
	OriginalURL string    `json:"original_url"`
	ShortCode   string    `json:"short_code"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
}

type URLRepository interface {
	Create(ctx context.Context, url *URL) error
	GetByShortCode(ctx context.Context, code string) (*URL, error)
}

type URLCache interface {
	Get(ctx context.Context, key string) (*URL, error)
	Set(ctx context.Context, key string, value *URL) error
}
