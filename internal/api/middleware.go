package api

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"goprl/internal/domain"

	"github.com/google/uuid"
)

type requestIDKey struct{}

// RequestID returns the ID assigned to the request, or an empty string.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.NewString()
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Info("request", "method", r.Method, "url", r.URL, "request_id", RequestID(r.Context()))
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimiter implements a fixed-window request limit.
type RateLimiter interface {
	Allow(context.Context, string, int, time.Duration) error
}

func RateLimitMiddleware(limiter RateLimiter, limit int, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if limit <= 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" || r.URL.Path == "/ready" {
				next.ServeHTTP(w, r)
				return
			}
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				// A bare IP is useful for in-process callers. Keep other malformed
				// addresses separate rather than grouping them under an empty key.
				ip = r.RemoteAddr
			}
			err = limiter.Allow(r.Context(), ip, limit, time.Minute)
			if errors.Is(err, domain.ErrRateLimitExceeded) {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			if err != nil {
				logger.Warn("rate limiter unavailable; allowing request", "error", err)
			}
			next.ServeHTTP(w, r)
		})
	}
}
