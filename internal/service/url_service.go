package service

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"time"

	"goprl/internal/domain"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

type URLService struct {
	store  domain.URLStore
	cache  domain.URLCache
	logger *slog.Logger
}

func NewURLService(store domain.URLStore, cache domain.URLCache, logger *slog.Logger) *URLService {
	return &URLService{
		store:  store,
		cache:  cache,
		logger: logger,
	}
}

func (s *URLService) Shorten(ctx context.Context, originalURL string) (*domain.URL, error) {
	var url *domain.URL
	var shortURL string
	var success bool
	for i := 0; i < 3; i++ {
		shortURL = generateShortURL(6)

		url = &domain.URL{
			OriginalURL: originalURL,
			ShortURL:    shortURL,
			CreatedAt:   time.Now(),
		}

		if err := s.store.CreateURL(ctx, url); errors.Is(err, domain.ErrCollision) {
			continue
		} else if err != nil {
			return nil, err
		}
		success = true
		break
	}
	if !success {
		return nil, errors.New("Failed to shorten URL")
	}
	if err := s.cache.Set(ctx, shortURL, url); err != nil {
		s.logger.Error("Failed to set cache", "error", err)

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
		return nil, errors.New("URL not found")
	}

	// Add to cache for next time
	_ = s.cache.Set(ctx, code, url)

	return url, nil
}

// Takes random chars from charset
// Future convert to base62
func generateShortURL(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
