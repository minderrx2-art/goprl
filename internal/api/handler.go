package api

import (
	"context"
	"strings"

	"goprl/internal/domain"
)

// URLService supplies the operations used by the HTTP handlers.
type URLService interface {
	Shorten(context.Context, string) (*domain.URL, error)
	Resolve(context.Context, string) (*domain.URL, error)
}

// Pinger checks a dependency's availability.
type Pinger interface {
	Ping(context.Context) error
}

type Handler struct {
	service URLService
	db      Pinger
	cache   Pinger
	baseURL string
}

func New(service URLService, db Pinger, cache Pinger, baseURL string) *Handler {
	return &Handler{service: service, db: db, cache: cache, baseURL: strings.TrimRight(baseURL, "/")}
}
