package service

import (
	"context"
	"errors"
	"goprl/internal/domain"
	"log/slog"
	"net/url"
	"strings"
	"time"
)

type URLService struct {
	store   domain.URLStore
	cache   domain.URLCache
	logger  *slog.Logger
	baseURL string
}

// URL service factory
func NewURLService(store domain.URLStore, cache domain.URLCache, logger *slog.Logger, baseURL string) *URLService {
	return &URLService{
		store:   store,
		cache:   cache,
		logger:  logger,
		baseURL: baseURL,
	}
}

func (s *URLService) Shorten(ctx context.Context, originalURL string) (*domain.URL, error) {
	validURL, err := validateUrl(originalURL)
	if err != nil {
		return nil, err
	}
	var url *domain.URL
	var shortURL string
	counter, _ := s.cache.Increment(ctx, "counter")
	shortURL = generateBase62(counter)
	url = &domain.URL{
		OriginalURL: validURL,
		ShortURL:    shortURL,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}

	if err := s.store.CreateURL(ctx, url); err != nil {
		return nil, err
	}

	if err := s.cache.Set(ctx, shortURL, url); err != nil {
		s.logger.Error("Failed to set cache", "error", err)

	}
	// Apply baseURL to shortURL for handler
	url.ShortURL = s.baseURL + "/" + shortURL
	return url, nil
}

func (s *URLService) Resolve(ctx context.Context, code string) (*domain.URL, error) {
	// Fast cache poke
	url, err := s.cache.Get(ctx, code)
	if err == nil && url != nil {
		s.logger.Info("Cache hit", "code", code)
		return url, nil
	} else {
		s.logger.Info("Cache miss", "code", code)
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

const charset = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// Big-endian
func generateBase62(num int64) string {
	if num == 0 {
		return string(charset[0])
	}
	base62_chars := [12]byte{}
	i := 11
	for num > 0 {
		rem := num % 62
		num /= 62
		base62_chars[i] = charset[rem]
		i--
	}
	return string(base62_chars[i+1:])
}

func validateUrl(link string) (string, error) {
	if !strings.Contains(link, "://") {
		link = "https://" + link
	}
	if !strings.Contains(link, "www.") {
		link = strings.Replace(link, "https://", "https://www.", 1)
	}
	u, err := url.Parse(link)
	if err != nil {
		return "", domain.ErrInvalidURL
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", domain.ErrInvalidScheme
	}
	host := u.Hostname()
	if host == "" {
		return "", domain.ErrInvalidURL
	}
	if len(strings.Split(host, ".")) != 3 {
		return "", domain.ErrInvalidURL
	}
	if len(host) < 4 {
		return "", domain.ErrInvalidURL
	}
	return link, nil
}
