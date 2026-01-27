package app

import (
	"context"
	"fmt"
	"goprl/internal/api"
	"goprl/internal/config"
	"goprl/internal/service"
	"goprl/internal/store"
	"goprl/internal/store/postgres"
	"goprl/internal/store/redis"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
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

func (a *app) Run() error {
	mux := http.NewServeMux()
	a.Handler.RegisterRoutes(mux)
	srv := &http.Server{
		Addr:    ":" + a.Config.Port,
		Handler: api.RequestIDMiddleware(api.LoggingMiddleware(a.Logger)(mux)),
	}
	srvErrors := make(chan error, 1)
	signalChan := make(chan os.Signal, 1)

	signal.Notify(signalChan, os.Interrupt)
	signal.Notify(signalChan, syscall.SIGTERM)

	go func() {
		a.Logger.Info("URL Shortener starting on http://localhost:" + a.Config.Port)
		// Wait for error and push it onto error channel
		srvErrors <- srv.ListenAndServe()
	}()

	// Blocks and waits for any of the selected channels to send a value
	select {
	case err := <-srvErrors:
		a.Logger.Error("server listener failure", "error", err)
		return err
	case sig := <-signalChan:
		a.Logger.Info("shutdown signal received", "signal", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			a.Logger.Error("server shutdown failed", "error", err)
			return err
		}
	}
	return nil
}

func (a *app) Close() {
	a.PostgresStore.Close()
	a.RedisStore.Close()
}
