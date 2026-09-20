package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"goprl/internal/domain"
)

const maxBytes = 4096

func (h *Handler) handleShorten(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}

	url, err := h.service.Shorten(r.Context(), req.URL)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidURL), errors.Is(err, domain.ErrInvalidScheme):
			http.Error(w, "invalid URL", http.StatusBadRequest)
		case errors.Is(err, domain.ErrURLAlreadyExists):
			http.Error(w, "URL already exists", http.StatusConflict)
		case errors.Is(err, domain.ErrRateLimitExceeded):
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"short_url":  url.ShortURL,
		"expires_at": url.ExpiresAt.String(),
	})
}
