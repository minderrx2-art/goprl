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

	fmt.Println("URL Shortener starting on http://localhost:" + app.config.port)
	if err := http.ListenAndServe(":"+app.config.port,
		api.RequestIDMiddleware(
			api.LoggingMiddleware(app.logger)(mux),
		)); err != nil {
		panic(err)
	}
}

type config struct {
	databaseURL string
	redisURL    string
	port        string
}

func LoadConfig() (*config, error) {
	_ = godotenv.Load()
	var databaseURL, redisURL, port string
	if port = os.Getenv("PORT"); port == "" {
		port = "8080"
	}
	if databaseURL = os.Getenv("DATABASE_URL"); databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}
	if redisURL = os.Getenv("REDIS_URL"); redisURL == "" {
		return nil, fmt.Errorf("REDIS_URL is not set")
	}
	return &config{
		databaseURL: databaseURL,
		redisURL:    redisURL,
		port:        port,
	}, nil
}

type app struct {
	postgresStore *postgres.Store
	redisStore    *redis.Cache
	logger        *slog.Logger
	handler       *api.Handler
	config        *config
}

func buildApp() *app {
	config, err := LoadConfig()
	if err != nil {
		panic("ENV: " + err.Error())
	}

	postgresStore, err := store.NewPostgresStore(config.databaseURL)
	if err != nil {
		panic("POSTGRES: " + err.Error())
	}

	redisStore, err := store.NewRedisStore(config.redisURL)
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
		config:        config,
	}
}

func (a *app) close() {
	a.postgresStore.Close()
	a.redisStore.Close()
}
