package api

import (
	"context"
	"goprl/internal/service"
)

type Handler struct {
	service *service.URLService
	db      Pinger
	cache   Pinger
}

type Pinger interface {
	Ping(ctx context.Context) error
}

func NewHandler(service *service.URLService, db Pinger, cache Pinger) *Handler {
	return &Handler{
		service: service,
		db:      db,
		cache:   cache,
	}
}
