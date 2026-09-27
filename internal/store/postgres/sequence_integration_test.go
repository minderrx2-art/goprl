package postgres_test

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"goprl/internal/service"
	"goprl/internal/store"
	"goprl/internal/store/postgres"
)

// Uses a private schema; GOPRL_TEST_DATABASE_URL must permit creating schemas.
func TestSequenceIntegration(t *testing.T) {
	dsn := os.Getenv("GOPRL_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set GOPRL_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	initSQL, err := os.ReadFile("../../../scripts/init.sql")
	if err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../../scripts/migrate_short_code_sequence.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing=%v", existing), func(t *testing.T) {
			schema := fmt.Sprintf("sequence_test_%d", time.Now().UnixNano())
			if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
				t.Fatal(err)
			}
			defer admin.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
			u, err := url.Parse(dsn)
			if err != nil {
				t.Fatal(err)
			}
			q := u.Query()
			q.Set("search_path", schema)
			u.RawQuery = q.Encode()
			db, err := sql.Open("pgx", u.String())
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if _, err := db.ExecContext(ctx, string(initSQL)); err != nil {
				t.Fatal(err)
			}
			wantFirst := int64(1)
			if existing {
				if _, err := db.ExecContext(ctx, "DROP SEQUENCE short_code_seq; INSERT INTO urls (id, short_code, original_url) VALUES (1000000, 'jU', 'https://old.example.com'), (2, '10', 'https://other.example.com'), (3, 'custom-code', 'https://custom.example.com'), (4, 'ZZZZZZZZZZZ', 'https://large.example.com'); SELECT setval('urls_id_seq', 1000000)"); err != nil {
					t.Fatal(err)
				}
				wantFirst = 1235
			}
			if _, err := db.ExecContext(ctx, string(migration)); err != nil {
				t.Fatal(err)
			}
			pg := postgres.New(db)
			id, err := pg.NextShortCodeID(ctx)
			if err != nil || id != wantFirst {
				t.Fatalf("first=%d want=%d error=%v", id, wantFirst, err)
			}
			if _, err := db.ExecContext(ctx, string(migration)); err != nil {
				t.Fatal(err)
			}
			id, err = pg.NextShortCodeID(ctx)
			if err != nil || id != wantFirst+1 {
				t.Fatalf("migration reset sequence: id=%d error=%v", id, err)
			}
			if existing {
				link, err := pg.GetByShortCode(ctx, "jU")
				if err != nil || link.ID != 1000000 {
					t.Fatalf("existing link changed: link=%v error=%v", link, err)
				}
			}
			// Two service instances share PostgreSQL but have independent Bloom filters.
			svcs := make([]*service.URLService, 2)
			for i := range svcs {
				svcs[i] = service.New(pg, nil, pg, store.NewBloomFilter(10000, 3), slog.New(slog.NewTextHandler(io.Discard, nil)))
			}
			const count = 64
			codes := make(chan string, count)
			errs := make(chan error, count)
			var wg sync.WaitGroup
			for i := range count {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					link, err := svcs[i%2].Shorten(ctx, fmt.Sprintf("https://example.com/%d", i))
					if err != nil {
						errs <- err
						return
					}
					got, err := svcs[(i+1)%2].Resolve(ctx, link.ShortCode)
					if err != nil {
						errs <- err
						return
					}
					if got.OriginalURL != link.OriginalURL {
						errs <- fmt.Errorf("resolve returned %q", got.OriginalURL)
						return
					}
					codes <- link.ShortCode
				}(i)
			}
			wg.Wait()
			close(codes)
			close(errs)
			for err := range errs {
				t.Error(err)
			}
			seen := make(map[string]bool)
			for code := range codes {
				if seen[code] {
					t.Errorf("duplicate code %q", code)
				}
				seen[code] = true
			}
			if len(seen) != count {
				t.Errorf("created %d links, want %d", len(seen), count)
			}
		})
	}
}
