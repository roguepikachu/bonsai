//go:build integration

package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/roguepikachu/bonsai/internal/domain"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// startPostgres spins up a Postgres container using testcontainers.
func startPostgres(ctx context.Context, t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	pg, err := tcpostgres.RunContainer(ctx,
		tcpostgres.WithUsername("bonsai"),
		tcpostgres.WithPassword("secret"),
		tcpostgres.WithDatabase("bonsai"),
	)
	if err != nil {
		t.Skipf("skipping: cannot start postgres container (is Docker running?): %v", err)
		return nil, func() {}
	}
	// Build DSN compatible with pgxpool
	host, _ := pg.Host(ctx)
	port, _ := pg.MappedPort(ctx, "5432")
	dsn := fmt.Sprintf("postgres://bonsai:secret@%s:%s/bonsai?sslmode=disable", host, port.Port())
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	// Increase timeout for slow CI environments
	cfg.MaxConnLifetime = 0
	cfg.MaxConnIdleTime = 0
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	// Wait until healthy
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		if err := pool.Ping(ctx); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("timeout waiting for db ready: %v", ctx.Err())
		case <-time.After(250 * time.Millisecond):
		}
	}
	cleanup := func() {
		pool.Close()
		_ = pg.Terminate(context.Background())
	}
	return pool, cleanup
}

func TestPostgresRepository_CRUDAndList(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool, cleanup := startPostgres(ctx, t)
	defer cleanup()

	repo := NewSnippetRepository(pool)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	s1 := domainSnippet("a1", now, nil, []string{"go", "notes"})
	s2 := domainSnippet("b2", now.Add(1*time.Second), nil, []string{"go"})
	exp := now.Add(10 * time.Minute)
	s3 := domainSnippet("c3", now.Add(2*time.Second), &exp, []string{"rust"})

	// Insert snippets
	if err := repo.Insert(ctx, s1); err != nil {
		t.Fatalf("insert s1: %v", err)
	}
	if err := repo.Insert(ctx, s2); err != nil {
		t.Fatalf("insert s2: %v", err)
	}
	if err := repo.Insert(ctx, s3); err != nil {
		t.Fatalf("insert s3: %v", err)
	}

	// FindByID
	got, err := repo.FindByID(ctx, "a1")
	if err != nil {
		t.Fatalf("find a1: %v", err)
	}
	if got.ID != "a1" || got.Content != s1.Content {
		t.Fatalf("find mismatch: %+v", got)
	}

	// List all (order by created_at desc)
	all, err := repo.List(ctx, 1, 10, "")
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("want 3, got %d", len(all))
	}
	if !(all[0].ID == "c3" && all[1].ID == "b2" && all[2].ID == "a1") {
		t.Fatalf("unexpected order: %v, %v, %v", all[0].ID, all[1].ID, all[2].ID)
	}

	// List filtered by tag
	goOnly, err := repo.List(ctx, 1, 10, "go")
	if err != nil {
		t.Fatalf("list go: %v", err)
	}
	if len(goOnly) != 2 {
		t.Fatalf("want 2 go-tagged, got %d", len(goOnly))
	}

	// Pagination
	page1, err := repo.List(ctx, 1, 2, "")
	if err != nil {
		t.Fatalf("list page1: %v", err)
	}
	page2, err := repo.List(ctx, 2, 2, "")
	if err != nil {
		t.Fatalf("list page2: %v", err)
	}
	if len(page1) != 2 || len(page2) != 1 {
		t.Fatalf("pagination wrong: p1=%d p2=%d", len(page1), len(page2))
	}
}

// domainSnippet is a tiny helper to build domain.Snippet for tests.
func domainSnippet(id string, created time.Time, expires *time.Time, tags []string) domain.Snippet {
	s := domain.Snippet{ID: id, Content: fmt.Sprintf("content-%s", id), CreatedAt: created, Tags: tags}
	if expires != nil {
		s.ExpiresAt = *expires
	}
	return s
}
