package api

import (
	"encoding/json"
	"net/http"

	"goprl/internal/buildinfo"
)

type healthResponse struct {
	Status string `json:"status"`
	buildinfo.Info
}

func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(healthResponse{Status: "ok", Info: buildinfo.Current()})
}
