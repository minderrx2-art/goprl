package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"goprl/internal/domain"
	"goprl/internal/service"
)

type Handler struct {
	service *service.URLService
}

func NewHandler(service *service.URLService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /shorten", h.handleShorten)
	mux.HandleFunc("GET /{code}", h.handleResolve)
	mux.HandleFunc("GET /health", h.handleHealth)
}

func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (h *Handler) handleShorten(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
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

func (h *Handler) handleResolve(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	url, err := h.service.Resolve(r.Context(), code)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrURLNotFound):
			http.Error(w, "URL not found", http.StatusNotFound)
		case errors.Is(err, domain.ErrURLExpired):
			http.Error(w, "URL expired", http.StatusGone)
		default:
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
		return
	}
	// 301 StatusMovedPermanently, caches redirect and skips server entirely on subsequent requests
	http.Redirect(w, r, url.OriginalURL, http.StatusTemporaryRedirect)
}
