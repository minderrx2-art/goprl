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
	postgresStore *postgres.Store
	redisStore    *redis.Cache
	logger        *slog.Logger
	handler       *api.Handler
	config        *config.Config
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

	bloom := store.NewBloomFilter(1000000, 3)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	service := service.NewURLService(postgresStore, redisStore, bloom, logger, config.BaseURL)
	handler := api.NewHandler(service)

	return &app{
		postgresStore: postgresStore,
		redisStore:    redisStore,
		logger:        logger,
		handler:       handler,
		config:        config,
	}, nil
}

func (a *app) Run() error {
	mux := http.NewServeMux()
	a.handler.RegisterRoutes(mux)
	srv := &http.Server{
		Addr:    ":" + a.config.Port,
		Handler: api.RequestIDMiddleware(api.LoggingMiddleware(a.logger)(api.RateLimitMiddleware(a.redisStore, a.config)(mux))),
	}
	srvErrors := make(chan error, 1)
	signalChan := make(chan os.Signal, 1)

	signal.Notify(signalChan, os.Interrupt)
	signal.Notify(signalChan, syscall.SIGTERM)

	if a.config.Env == "prod" {
		go func() {
			srvErrors <- srv.ListenAndServeTLS(
				"/etc/letsencrypt/live/goprl.co.uk/fullchain.pem",
				"/etc/letsencrypt/live/goprl.co.uk/privkey.pem",
			)
		}()
	} else {
		go func() {
			srvErrors <- srv.ListenAndServe()
		}()
	}

	// Blocks and waits for any of the selected channels to send a value
	select {
	case err := <-srvErrors:
		a.logger.Error("server listener failure", "error", err)
		return err
	case sig := <-signalChan:
		a.logger.Info("shutdown signal received", "signal", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			a.logger.Error("server shutdown failed", "error", err)
			return err
		}
	}
	return nil
}

func (a *app) Close() {
	a.postgresStore.Close()
	a.redisStore.Close()
}
