package api

import (
	"context"
	"log/slog"
	"net/http"

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

func generateRandomID() string {
	return uuid.NewString()
}
