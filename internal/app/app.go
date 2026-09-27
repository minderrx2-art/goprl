package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"goprl/internal/api"
	"goprl/internal/config"
	"goprl/internal/service"
	"goprl/internal/store"
	"goprl/internal/store/postgres"
	"goprl/internal/store/redis"
)

type App struct {
	postgresStore *postgres.Store
	redisStore    *redis.Cache
	logger        *slog.Logger
	handler       *api.Handler
	config        *config.Config
}

func New(cfg *config.Config) (*App, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return newWithStores(ctx, cfg, store.OpenPostgres, store.OpenRedis)
}

func newWithStores(ctx context.Context, cfg *config.Config,
	openPostgres func(context.Context, string) (*postgres.Store, error),
	openRedis func(context.Context, string) (*redis.Cache, error),
) (*App, error) {
	postgresStore, err := openPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	redisStore, err := openRedis(ctx, cfg.RedisURL)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("open redis: %w", err), postgresStore.Close())
	}
	bloom := store.NewBloomFilter(1000000, 3)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	svc := service.New(postgresStore, redisStore, redisStore, bloom, logger)
	return &App{
		postgresStore: postgresStore,
		redisStore:    redisStore,
		logger:        logger,
		handler:       api.New(svc, postgresStore, redisStore, cfg.BaseURL),
		config:        cfg,
	}, nil
}

// Run serves requests until the context is canceled or the listener fails.
func (a *App) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	a.handler.RegisterRoutes(mux)
	srv := &http.Server{
		Addr:              ":" + a.config.Port,
		Handler:           api.RequestIDMiddleware(api.LoggingMiddleware(a.logger)(api.RateLimitMiddleware(a.redisStore, a.config.RateLimit, a.logger)(mux))),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	listen := srv.ListenAndServe
	if a.config.Env == "prod" {
		listen = func() error {
			return srv.ListenAndServeTLS(
				"/etc/letsencrypt/live/goprl.co.uk/fullchain.pem",
				"/etc/letsencrypt/live/goprl.co.uk/privkey.pem",
			)
		}
	}
	return runServer(ctx, srv, listen, 5*time.Second)
}

func runServer(ctx context.Context, srv *http.Server, listen func() error, shutdownTimeout time.Duration) error {
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- listen() }()
	select {
	case err := <-serverErrors:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return errors.Join(fmt.Errorf("serve HTTP: %w", err), srv.Close())
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return errors.Join(fmt.Errorf("shut down HTTP: %w", err), srv.Close())
		}
		return nil
	}
}

// Close releases both stores, even if closing the first one fails.
func (a *App) Close() error {
	return errors.Join(a.postgresStore.Close(), a.redisStore.Close())
}
