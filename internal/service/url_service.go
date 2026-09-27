package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"goprl/internal/domain"

	"golang.org/x/net/publicsuffix"
)

// Store persists short links.
type Store interface {
	CreateURL(context.Context, *domain.URL) error
	GetByShortCode(context.Context, string) (*domain.URL, error)
	GetByOriginalURL(context.Context, string) (*domain.URL, error)
}

// Cache provides best-effort link caching.
type Cache interface {
	GetURL(context.Context, string) (*domain.URL, error)
	SetURL(context.Context, string, *domain.URL) error
}

// Counter allocates monotonically increasing short-code values.
type Counter interface {
	Increment(context.Context, string) (int64, error)
}

// Bloom tracks original URLs seen by this process.
type Bloom interface {
	Add(string)
	Contains(string) bool
}

type URLService struct {
	store   Store
	cache   Cache
	counter Counter
	bloom   Bloom
	logger  *slog.Logger
}

func New(store Store, cache Cache, counter Counter, bloom Bloom, logger *slog.Logger) *URLService {
	return &URLService{store: store, cache: cache, counter: counter, bloom: bloom, logger: logger}
}

const (
	linkLifetime          = 7 * 24 * time.Hour
	cacheWriteTimeout     = 250 * time.Millisecond
	maxAllocationAttempts = 8
)

func (s *URLService) Shorten(ctx context.Context, originalURL string) (*domain.URL, error) {
	validURL, err := validateURL(originalURL)
	if err != nil {
		return nil, err
	}
	if s.bloom.Contains(validURL) {
		link, err := s.cache.GetURL(ctx, validURL)
		if err == nil && link != nil && !isExpired(link, time.Now()) {
			return link, nil
		}
		link, err = s.store.GetByOriginalURL(ctx, validURL)
		if err != nil && !errors.Is(err, domain.ErrURLNotFound) {
			return nil, fmt.Errorf("look up original URL: %w", err)
		}
		if err == nil && link != nil && !isExpired(link, time.Now()) {
			s.cacheLink(ctx, link, validURL)
			return link, nil
		}
	}
	for range maxAllocationAttempts {
		counter, err := s.counter.Increment(ctx, "counter")
		if err != nil {
			return nil, fmt.Errorf("allocate short code: %w", err)
		}
		if counter <= 0 {
			return nil, fmt.Errorf("allocate short code: counter must be positive, got %d", counter)
		}
		now := time.Now()
		link := &domain.URL{
			OriginalURL: validURL,
			ShortCode:   generateBase62(counter),
			CreatedAt:   now,
			ExpiresAt:   now.Add(linkLifetime),
		}
		if err := s.store.CreateURL(ctx, link); err != nil {
			if errors.Is(err, domain.ErrURLAlreadyExists) {
				s.logger.Warn("short code collision", "code", link.ShortCode)
				continue
			}
			return nil, fmt.Errorf("create short link: %w", err)
		}
		s.cacheLink(ctx, link, link.ShortCode, validURL)
		s.bloom.Add(validURL)
		return link, nil
	}
	return nil, fmt.Errorf("allocate short code after %d attempts: %w", maxAllocationAttempts, domain.ErrURLAlreadyExists)
}

func (s *URLService) Resolve(ctx context.Context, code string) (*domain.URL, error) {
	link, err := s.cache.GetURL(ctx, code)
	if err == nil && link != nil {
		if isExpired(link, time.Now()) {
			return nil, domain.ErrURLExpired
		}
		return link, nil
	}
	link, err = s.store.GetByShortCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("resolve short code: %w", err)
	}
	if link == nil {
		return nil, domain.ErrURLNotFound
	}
	if isExpired(link, time.Now()) {
		return nil, domain.ErrURLExpired
	}
	s.cacheLink(ctx, link, code)
	return link, nil
}

func (s *URLService) cacheLink(ctx context.Context, link *domain.URL, keys ...string) {
	ctx, cancel := context.WithTimeout(ctx, cacheWriteTimeout)
	defer cancel()
	for _, key := range keys {
		if err := s.cache.SetURL(ctx, key, link); err != nil {
			s.logger.Warn("cache write failed", "error", err)
		}
	}
}

// A zero timestamp represents a link without expiry. The deadline itself is expired.
func isExpired(link *domain.URL, now time.Time) bool {
	return !link.ExpiresAt.IsZero() && !now.Before(link.ExpiresAt)
}

const charset = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func generateBase62(num int64) string {
	if num == 0 {
		return string(charset[0])
	}
	var digits [11]byte // Enough for the largest positive int64 in base 62.
	i := len(digits)
	for num > 0 {
		i--
		digits[i] = charset[num%62]
		num /= 62
	}
	return string(digits[i:])
}

func validateURL(link string) (string, error) {
	if !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") {
		link = "https://" + link
	}
	u, err := url.Parse(link)
	if err != nil || u.Hostname() == "" {
		return "", domain.ErrInvalidURL
	}
	if _, err := publicsuffix.EffectiveTLDPlusOne(u.Hostname()); err != nil {
		return "", domain.ErrInvalidURL
	}
	return link, nil
}
