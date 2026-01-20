package main

import (
	"fmt"
	"goprl/internal/api"
	"goprl/internal/service"
	"goprl/internal/store"
	"net/http"
)

func main() {
	postgresStore, err := store.NewPostgresStore("postgres://minvy:pass@localhost:5432/goprl")
	if err != nil {
		panic(err)
	}
	redisStore, err := store.NewRedisStore("redis://:pass@localhost:6379/0")
	if err != nil {
		panic(err)
	}

	defer postgresStore.Close()
	defer redisStore.Close()

	service := service.NewURLService(postgresStore, redisStore)
	handler := api.NewHandler(service)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	fmt.Println("URL Shortener starting on http://localhost:8080")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		panic(err)
	}

}
