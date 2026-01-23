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

type app struct {
	postgresStore *postgres.Store
	redisStore    *redis.Cache
	logger        *slog.Logger
	handler       *api.Handler
}

func buildApp() *app {
	postgresStore, err := store.NewPostgresStore("postgres://minvy:pass@localhost:5432/goprl")
	if err != nil {
		panic("POSTGRES: " + err.Error())
	}
	redisStore, err := store.NewRedisStore("redis://:pass@localhost:6379/0")
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
