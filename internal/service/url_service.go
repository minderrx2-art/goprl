package service

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"goprl/internal/domain"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

type URLService struct {
	store domain.URLStore
	cache domain.URLCache
}

func NewURLService(store domain.URLStore, cache domain.URLCache) *URLService {
	return &URLService{
		store: store,
		cache: cache,
	}
}

func (s *URLService) Shorten(ctx context.Context, originalURL string) (*domain.URL, error) {
	ShortURL := generateShortURL(6)

	url := &domain.URL{
		OriginalURL: originalURL,
		ShortURL:    ShortURL,
		CreatedAt:   time.Now(),
	}

	// 1. Save to database
	if err := s.store.CreateURL(ctx, url); err != nil {
		return nil, err
	}

	// 2. Update cache
	if err := s.cache.Set(ctx, ShortURL, url); err != nil {
		// Add logging later
	}

	return url, nil
}

func (s *URLService) Resolve(ctx context.Context, code string) (*domain.URL, error) {
	// Fast cache poke
	url, err := s.cache.Get(ctx, code)
	if err == nil && url != nil {
		return url, nil
	}

	// Slow database lookup
	url, err = s.store.GetByShortURL(ctx, code)
	if err != nil {
		return nil, errors.New("url not found")
	}

	// Add to cache for next time
	_ = s.cache.Set(ctx, code, url)

	return url, nil
}

// Takes random chars from charset
func generateShortURL(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
