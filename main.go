package main

import (
	"database/sql"
	"fmt"
	"goprl/internal/api"
	"goprl/internal/service"
	"goprl/internal/store/postgres"
	"goprl/internal/store/redis"
	"net/http"

	goredis "github.com/go-redis/redis/v8"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	db, err := sql.Open("pgx", "postgres://minvy:pass@localhost:5432/goprl")
	if err != nil {
		panic(err)
	}
	store := postgres.NewStore(db)

	opt, err := goredis.ParseURL("redis://:pass@localhost:6379/0")
	if err != nil {
		panic(err)
	}
	rdb := goredis.NewClient(opt)
	cache := redis.NewCache(rdb)

	service := service.NewURLService(store, cache)
	handler := api.NewHandler(service)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	fmt.Println("URL Shortener starting on http://localhost:8080")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		panic(err)
	}

}
