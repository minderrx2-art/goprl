package api

import (
	"errors"
	"net/http"

	"goprl/internal/domain"
)

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
	// 307 avoids browsers permanently caching the redirect past URL expiry.
	http.Redirect(w, r, url.OriginalURL, http.StatusTemporaryRedirect)
}
