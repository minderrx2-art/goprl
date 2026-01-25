package main

import (
	"fmt"
	"goprl/internal/api"
	"goprl/internal/service"
	"goprl/internal/store"
	"goprl/internal/store/postgres"
	"goprl/internal/store/redis"

	"log/slog"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	app := buildApp()
	defer app.close()

	mux := http.NewServeMux()
	app.handler.RegisterRoutes(mux)

	fmt.Println("URL Shortener starting on http://localhost:8080")
	if err := http.ListenAndServe(":8080",
		api.RequestIDMiddleware(
			api.LoggingMiddleware(app.logger)(mux),
		)); err != nil {
		panic(err)
	}
}

type Config struct {
	DatabaseURL string
	RedisURL    string
}

func LoadConfig() (*Config, error) {
	_ = godotenv.Load()
	var DatabaseURL, RedisURL string
	if DatabaseURL = os.Getenv("DATABASE_URL"); DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}
	if RedisURL = os.Getenv("REDIS_URL"); RedisURL == "" {
		return nil, fmt.Errorf("REDIS_URL is not set")
	}
	return &Config{
		DatabaseURL: DatabaseURL,
		RedisURL:    RedisURL,
	}, nil
}

type app struct {
	postgresStore *postgres.Store
	redisStore    *redis.Cache
	logger        *slog.Logger
	handler       *api.Handler
}

func buildApp() *app {
	config, err := LoadConfig()
	if err != nil {
		panic("ENV: " + err.Error())
	}

	postgresStore, err := store.NewPostgresStore(config.DatabaseURL)
	if err != nil {
		panic("POSTGRES: " + err.Error())
	}

	redisStore, err := store.NewRedisStore(config.RedisURL)
	if err != nil {
		panic("REDIS: " + err.Error())
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	service := service.NewURLService(postgresStore, redisStore, logger)
	handler := api.NewHandler(service)
	return &app{
		postgresStore: postgresStore,
		redisStore:    redisStore,
		logger:        logger,
		handler:       handler,
	}
}

func (a *app) close() {
	a.postgresStore.Close()
	a.redisStore.Close()
}
