package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"goprl/internal/store/postgres"
	"goprl/internal/store/redis"

	goredis "github.com/go-redis/redis/v8"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func OpenPostgres(ctx context.Context, url string) (*postgres.Store, error) {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return pingPostgres(ctx, db)
}

func pingPostgres(ctx context.Context, db *sql.DB) (*postgres.Store, error) {
	if err := db.PingContext(ctx); err != nil {
		return nil, errors.Join(fmt.Errorf("ping database: %w", err), db.Close())
	}
	return postgres.New(db), nil
}

func OpenRedis(ctx context.Context, url string) (*redis.Cache, error) {
	opt, err := goredis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}
	return pingRedis(ctx, goredis.NewClient(opt))
}

func pingRedis(ctx context.Context, client *goredis.Client) (*redis.Cache, error) {
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("ping redis: %w", err), client.Close())
	}
	return redis.New(client), nil
}
