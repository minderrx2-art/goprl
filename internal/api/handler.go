package api

import (
	"encoding/json"
	"net/http"

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

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(url)
}

func (h *Handler) handleResolve(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	url, err := h.service.Resolve(r.Context(), code)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	// 302 StatusMovedPermanently, caches redirect and skips server entirely on subsequent requests
	http.Redirect(w, r, url.OriginalURL, http.StatusMovedPermanently)
}
