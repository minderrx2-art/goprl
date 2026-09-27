package domain

import (
	"errors"
	"time"
)

var (
	ErrURLNotFound       = errors.New("URL not found")
	ErrURLExpired        = errors.New("URL expired")
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	ErrInvalidURL        = errors.New("invalid URL")
	ErrURLAlreadyExists  = errors.New("URL already exists")
)

type URL struct {
	ID          int64     `json:"id"`
	OriginalURL string    `json:"original_url"`
	ShortCode   string    `json:"short_code"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at,omitempty"`
}
