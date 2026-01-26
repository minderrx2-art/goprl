package app

import (
	"fmt"
	"goprl/internal/api"
	"goprl/internal/config"
	"goprl/internal/service"
	"goprl/internal/store"
	"goprl/internal/store/postgres"
	"goprl/internal/store/redis"
	"log/slog"
	"os"
)

type app struct {
	PostgresStore *postgres.Store
	RedisStore    *redis.Cache
	Logger        *slog.Logger
	Handler       *api.Handler
	Config        *config.Config
}

func NewApp(config *config.Config) (*app, error) {
	postgresStore, err := store.NewPostgresStore(config.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("POSTGRES: %w", err)
	}

	redisStore, err := store.NewRedisStore(config.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("REDIS: %w", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	service := service.NewURLService(postgresStore, redisStore, logger)
	handler := api.NewHandler(service)

	return &app{
		PostgresStore: postgresStore,
		RedisStore:    redisStore,
		Logger:        logger,
		Handler:       handler,
		Config:        config,
	}, nil
}

func (a *app) Close() {
	a.PostgresStore.Close()
	a.RedisStore.Close()
}
