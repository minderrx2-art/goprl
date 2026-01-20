package main

import (
	"context"
	"database/sql"
	"fmt"
	"goprl/internal/api"
	"goprl/internal/domain"
	"goprl/internal/service"
	"goprl/internal/store/postgres"
	"net/http"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	db, err := sql.Open("pgx", "postgres://minvy:pass@localhost:5432/goprl")
	if err != nil {
		panic(err)
	}

	store := postgres.NewStore(db)
	mockCache := &mockCache{}

	service := service.NewURLService(store, mockCache)
	handler := api.NewHandler(service)

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	fmt.Println("URL Shortener starting on http://localhost:8080")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		panic(err)
	}

}

type mockCache struct{}

func (n *mockCache) Get(ctx context.Context, key string) (*domain.URL, error) {
	return nil, nil
}
func (n *mockCache) Set(ctx context.Context, key string, value *domain.URL) error {
	return nil
}
