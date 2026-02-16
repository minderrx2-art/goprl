package api

import (
	"context"
	"errors"
	"goprl/internal/config"
	"goprl/internal/domain"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type contextKey string

const RequestIDKey contextKey = "request_id"

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := generateRandomID()
		ctx := context.WithValue(r.Context(), RequestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Capture logger in closure and return the middleware handler
func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, _ := r.Context().Value(RequestIDKey).(string)
			logger.Info("Request", "method", r.Method, "url", r.URL, "request_id", id)
			next.ServeHTTP(w, r)
		})
	}
}

func RateLimitMiddleware(cache domain.URLCache, config *config.Config) func(http.Handler) http.Handler {
	if config.RateLimit <= 0 {
		return func(next http.Handler) http.Handler {
			return next
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			err := cache.Allow(r.Context(), ip, config.RateLimit, time.Minute)
			if errors.Is(err, domain.ErrRateLimitExceeded) {
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func generateRandomID() string {
	return uuid.NewString()
}
