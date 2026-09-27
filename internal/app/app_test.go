package app

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"goprl/internal/config"
	"goprl/internal/store/postgres"
	"goprl/internal/store/redis"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	goredis "github.com/go-redis/redis/v8"
)

func TestStartupFailureCleanup(t *testing.T) {
	startupErr := errors.New("connection failed")
	t.Run("postgres failure", func(t *testing.T) {
		_, err := newWithStores(context.Background(), &config.Config{},
			func(context.Context, string) (*postgres.Store, error) { return nil, startupErr },
			func(context.Context, string) (*redis.Cache, error) {
				t.Fatal("redis opened after postgres failure")
				return nil, nil
			},
		)
		if !errors.Is(err, startupErr) {
			t.Fatalf("lost startup error: %v", err)
		}
	})
	t.Run("redis failure", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		closeErr := errors.New("close failed")
		mock.ExpectClose().WillReturnError(closeErr)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err = newWithStores(ctx, &config.Config{},
			func(got context.Context, _ string) (*postgres.Store, error) {
				if got != ctx {
					t.Fatal("startup context not shared")
				}
				return postgres.New(db), nil
			},
			func(got context.Context, _ string) (*redis.Cache, error) {
				if got != ctx {
					t.Fatal("startup context not shared")
				}
				return nil, startupErr
			},
		)
		if !errors.Is(err, startupErr) || !errors.Is(err, closeErr) {
			t.Fatalf("lost startup or cleanup error: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestCloseReleasesBothStores(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	closeErr := errors.New("postgres close failed")
	mock.ExpectClose().WillReturnError(closeErr)
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	a := &App{postgresStore: postgres.New(db), redisStore: redis.New(client)}
	if err := a.Close(); !errors.Is(err, closeErr) {
		t.Fatalf("lost close error: %v", err)
	}
	if err := client.Ping(context.Background()).Err(); !errors.Is(err, goredis.ErrClosed) {
		t.Fatalf("redis not closed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRunServerListenerErrors(t *testing.T) {
	listenErr := errors.New("listen failed")
	for _, err := range []error{http.ErrServerClosed, listenErr} {
		got := runServer(context.Background(), &http.Server{}, func() error { return err }, time.Second)
		if errors.Is(err, http.ErrServerClosed) {
			if got != nil {
				t.Fatalf("normal close: %v", got)
			}
		} else if !errors.Is(got, err) {
			t.Fatalf("lost listener error: %v", got)
		}
	}
}

func TestRunServerCancellation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	srv := &http.Server{ReadHeaderTimeout: time.Second}
	t.Cleanup(func() { srv.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	listenResult := make(chan error, 1)
	result := make(chan error, 1)
	go func() {
		result <- runServer(ctx, srv, func() error {
			close(started)
			err := srv.Serve(listener)
			listenResult <- err
			return err
		}, time.Second)
	}()
	<-started
	cancel()
	if err := awaitError(t, result); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if err := awaitError(t, listenResult); !errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("listener remained open: %v", err)
	}
}

func TestRunServerShutdownTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	entered := make(chan struct{})
	handlerDone := make(chan struct{})
	srv := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		close(handlerDone)
	})}
	t.Cleanup(func() { srv.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		result <- runServer(ctx, srv, func() error { return srv.Serve(listener) }, 20*time.Millisecond)
	}()
	client := &http.Client{Timeout: time.Second}
	t.Cleanup(client.CloseIdleConnections)
	requestDone := make(chan error, 1)
	go func() {
		resp, err := client.Get("http://" + listener.Addr().String())
		if resp != nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		requestDone <- err
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not start")
	}
	cancel()
	if err := awaitError(t, result); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want shutdown deadline, got %v", err)
	}
	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout did not force-close active request")
	}
	awaitError(t, requestDone)
}

func awaitError(t *testing.T, ch <-chan error) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("operation did not finish")
		return nil
	}
}
