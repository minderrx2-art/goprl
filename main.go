package main

import (
	"fmt"
	"goprl/internal/api"
	"goprl/internal/service"
	"goprl/internal/store"
	"log/slog"
	"net/http"
	"os"
)

func main() {
	postgresStore, err := store.NewPostgresStore("postgres://minvy:pass@localhost:5432/goprl")
	if err != nil {
		panic("POSTGRES: " + err.Error())
	}
	redisStore, err := store.NewRedisStore("redis://:pass@localhost:6379/0")
	if err != nil {
		panic("REDIS: " + err.Error())
	}

	defer postgresStore.Close()
	defer redisStore.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	service := service.NewURLService(postgresStore, redisStore, logger)
	handler := api.NewHandler(service)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	fmt.Println("URL Shortener starting on http://localhost:8080")

	if err := http.ListenAndServe(":8080", api.RequestIDMiddleware(api.LoggingMiddleware(logger)(mux))); err != nil {
		panic(err)
	}
}
