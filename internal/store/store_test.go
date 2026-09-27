package store

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	goredis "github.com/go-redis/redis/v8"
)

func TestPostgresPingFailureClosesDatabase(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	pingErr := errors.New("database unavailable")
	mock.ExpectPing().WillReturnError(pingErr)
	mock.ExpectClose()
	store, err := pingPostgres(context.Background(), db)
	if store != nil || !errors.Is(err, pingErr) {
		t.Fatalf("store=%v error=%v", store, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCanceledPostgresStartup(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	mock.ExpectClose()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store, err := pingPostgres(ctx, db)
	if store != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("store=%v error=%v", store, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCanceledRedisStartupClosesClient(t *testing.T) {
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cache, err := pingRedis(ctx, client)
	if cache != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cache=%v error=%v", cache, err)
	}
	if err := client.Ping(context.Background()).Err(); !errors.Is(err, goredis.ErrClosed) {
		t.Fatalf("client not closed: %v", err)
	}
}
