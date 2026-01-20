package store

import (
	"context"
	"database/sql"
	"goprl/internal/store/postgres"
	"goprl/internal/store/redis"

	goredis "github.com/go-redis/redis/v8"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func NewPostgresStore(url string) (*postgres.Store, error) {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return postgres.NewStore(db), nil
}

func NewRedisStore(url string) (*redis.Cache, error) {
	opt, err := goredis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	rdb := goredis.NewClient(opt)
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return redis.NewCache(rdb), nil
}
