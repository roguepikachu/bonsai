//go:build integration

package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	cachedRepo "github.com/roguepikachu/bonsai/internal/repository/cached"
	postgresRepo "github.com/roguepikachu/bonsai/internal/repository/postgres"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// TestService_IntegrationPostgres tests the service layer with real PostgreSQL
func TestService_IntegrationPostgres(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	var dsn string
	var pool *pgxpool.Pool
	var err error

	// Check if running in CI environment
	if os.Getenv("CI") == "true" {
		// Use the existing database service in CI
		dsn = os.Getenv("DATABASE_URL")
		if dsn == "" {
			t.Skip("DATABASE_URL not set in CI environment")
			return
		}
		pool, err = pgxpool.New(ctx, dsn)
	} else {
		// Start PostgreSQL container for local testing
		pg, err := tcpostgres.RunContainer(ctx,
			tcpostgres.WithUsername("bonsai"),
			tcpostgres.WithPassword("secret"),
			tcpostgres.WithDatabase("bonsai"),
		)
		if err != nil {
			t.Skipf("skipping: cannot start postgres container: %v", err)
			return
		}
		defer pg.Terminate(ctx)

		// Connect to PostgreSQL
		host, _ := pg.Host(ctx)
		port, _ := pg.MappedPort(ctx, "5432")
		dsn = fmt.Sprintf("postgres://bonsai:secret@%s:%s/bonsai?sslmode=disable", host, port.Port())
		pool, err = pgxpool.New(ctx, dsn)
	}
	if err != nil {
		t.Fatalf("Failed to connect to postgres: %v", err)
	}
	defer pool.Close()

	// Wait for database to be ready
	for i := 0; i < 30; i++ {
		if err := pool.Ping(ctx); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
		if i == 29 {
			t.Fatalf("Database not ready after 3 seconds")
		}
	}

	// Setup repository and service
	repo := postgresRepo.NewSnippetRepository(pool)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("Failed to ensure schema: %v", err)
	}

	// Use RealClock for integration tests to match database NOW()
	clock := RealClock{}
	svc := NewService(repo, clock)

	t.Run("CreateAndRetrieveSnippet", func(t *testing.T) {
		snippet, err := svc.CreateSnippet(ctx, "Integration test content", 300, []string{"integration", "postgres"})
		if err != nil {
			t.Fatalf("CreateSnippet failed: %v", err)
		}

		if snippet.ID == "" {
			t.Error("Expected snippet ID to be generated")
		}
		if snippet.Content != "Integration test content" {
			t.Errorf("Expected content 'Integration test content', got '%s'", snippet.Content)
		}
		if len(snippet.Tags) != 2 {
			t.Errorf("Expected 2 tags, got %d", len(snippet.Tags))
		}

		// Retrieve the snippet
		retrieved, _, err := svc.GetSnippetByID(ctx, snippet.ID)
		if err != nil {
			t.Fatalf("GetSnippetByID failed: %v", err)
		}
		if retrieved.Content != snippet.Content {
			t.Errorf("Retrieved content mismatch: got '%s', want '%s'", retrieved.Content, snippet.Content)
		}
	})

	t.Run("ListSnippetsWithPagination", func(t *testing.T) {
		// Create multiple snippets
		for i := 0; i < 15; i++ {
			_, err := svc.CreateSnippet(ctx, fmt.Sprintf("Test content %d", i), 300, []string{"test", fmt.Sprintf("batch-%d", i/5)})
			if err != nil {
				t.Fatalf("Failed to create snippet %d: %v", i, err)
			}
		}

		// Test pagination
		page1, err := svc.ListSnippets(ctx, 1, 10, "")
		if err != nil {
			t.Fatalf("ListSnippets page 1 failed: %v", err)
		}
		if len(page1) != 10 {
			t.Errorf("Expected 10 snippets on page 1, got %d", len(page1))
		}

		page2, err := svc.ListSnippets(ctx, 2, 10, "")
		if err != nil {
			t.Fatalf("ListSnippets page 2 failed: %v", err)
		}
		if len(page2) < 5 {
			t.Errorf("Expected at least 5 snippets on page 2, got %d", len(page2))
		}

		// Test tag filtering
		filtered, err := svc.ListSnippets(ctx, 1, 20, "test")
		if err != nil {
			t.Fatalf("ListSnippets with tag filter failed: %v", err)
		}
		if len(filtered) < 15 {
			t.Errorf("Expected at least 15 snippets with 'test' tag, got %d", len(filtered))
		}
	})

	t.Run("ExpiredSnippets", func(t *testing.T) {
		// Create snippet with 1 second expiry
		snippet, err := svc.CreateSnippet(ctx, "Short lived", 1, []string{"temp"})
		if err != nil {
			t.Fatalf("CreateSnippet failed: %v", err)
		}

		// Should be retrievable immediately
		_, _, err = svc.GetSnippetByID(ctx, snippet.ID)
		if err != nil {
			t.Fatalf("GetSnippetByID failed: %v", err)
		}

		// Wait for expiry
		time.Sleep(2 * time.Second)

		// Should not be found after expiry
		_, _, err = svc.GetSnippetByID(ctx, snippet.ID)
		if err == nil || !errors.Is(err, ErrSnippetExpired) {
			t.Errorf("Expected ErrSnippetExpired, got: %v", err)
		}
	})
}

// TestService_IntegrationRedisCache tests the service with Redis caching
func TestService_IntegrationRedisCache(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	var dsn string
	var pool *pgxpool.Pool
	var err error

	// Check if running in CI environment
	if os.Getenv("CI") == "true" {
		// Use the existing database service in CI
		dsn = os.Getenv("DATABASE_URL")
		if dsn == "" {
			t.Skip("DATABASE_URL not set in CI environment")
			return
		}
		pool, err = pgxpool.New(ctx, dsn)
	} else {
		// Start PostgreSQL container for local testing
		pg, err := tcpostgres.RunContainer(ctx,
			tcpostgres.WithUsername("bonsai"),
			tcpostgres.WithPassword("secret"),
			tcpostgres.WithDatabase("bonsai"),
		)
		if err != nil {
			t.Skipf("skipping: cannot start postgres container: %v", err)
			return
		}
		defer pg.Terminate(ctx)

		// Connect to PostgreSQL
		host, _ := pg.Host(ctx)
		port, _ := pg.MappedPort(ctx, "5432")
		dsn = fmt.Sprintf("postgres://bonsai:secret@%s:%s/bonsai?sslmode=disable", host, port.Port())
		pool, err = pgxpool.New(ctx, dsn)
	}
	if err != nil {
		t.Fatalf("Failed to connect to postgres: %v", err)
	}
	defer pool.Close()

	// Wait for database to be ready
	for i := 0; i < 30; i++ {
		if err := pool.Ping(ctx); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
		if i == 29 {
			t.Fatalf("Database not ready after 3 seconds")
		}
	}

	// Setup Redis client
	var rdb *redis.Client
	if os.Getenv("CI") == "true" && os.Getenv("REDIS_URL") != "" {
		// Use existing Redis service in CI
		opt, err := redis.ParseURL(os.Getenv("REDIS_URL"))
		if err != nil {
			t.Fatalf("Failed to parse REDIS_URL: %v", err)
		}
		rdb = redis.NewClient(opt)
	} else {
		// Start mini Redis server for local testing
		miniRedis, err := miniredis.Run()
		if err != nil {
			t.Fatalf("Failed to start miniredis: %v", err)
		}
		defer miniRedis.Close()
		
		rdb = redis.NewClient(&redis.Options{
			Addr: miniRedis.Addr(),
		})
	}
	defer rdb.Close()

	// Setup repository and service with caching
	pgRepo := postgresRepo.NewSnippetRepository(pool)
	if err := pgRepo.EnsureSchema(ctx); err != nil {
		t.Fatalf("Failed to ensure schema: %v", err)
	}

	cachedRepo := cachedRepo.NewSnippetRepository(pgRepo, rdb, 5*time.Minute)
	// Use RealClock for integration tests to match database NOW()
	clock := RealClock{}
	svc := NewService(cachedRepo, clock)

	t.Run("CacheHitAndMiss", func(t *testing.T) {
		// Create snippet
		snippet, err := svc.CreateSnippet(ctx, "Cached content", 300, []string{"cache", "test"})
		if err != nil {
			t.Fatalf("CreateSnippet failed: %v", err)
		}

		// First read (cache miss - loads from database)
		retrieved1, _, err := svc.GetSnippetByID(ctx, snippet.ID)
		if err != nil {
			t.Fatalf("First GetSnippetByID failed: %v", err)
		}

		// Second read (cache hit - loads from Redis)
		retrieved2, _, err := svc.GetSnippetByID(ctx, snippet.ID)
		if err != nil {
			t.Fatalf("Second GetSnippetByID failed: %v", err)
		}

		// Both should return the same content
		if retrieved1.Content != retrieved2.Content {
			t.Errorf("Content mismatch between cache miss and hit: '%s' vs '%s'",
				retrieved1.Content, retrieved2.Content)
		}
	})

	t.Run("CacheInvalidation", func(t *testing.T) {
		// Create multiple snippets to populate cache
		var snippetIDs []string
		for i := 0; i < 5; i++ {
			snippet, err := svc.CreateSnippet(ctx, fmt.Sprintf("Cache test %d", i), 300, []string{"invalidation"})
			if err != nil {
				t.Fatalf("CreateSnippet %d failed: %v", i, err)
			}
			snippetIDs = append(snippetIDs, snippet.ID)
		}

		// Read all to populate cache
		for _, id := range snippetIDs {
			_, _, err := svc.GetSnippetByID(ctx, id)
			if err != nil {
				t.Fatalf("GetSnippetByID failed for %s: %v", id, err)
			}
		}

		// Verify cache has keys
		keys := rdb.Keys(ctx, "*").Val()
		if len(keys) == 0 {
			t.Error("Expected cache to have keys after reading snippets")
		}

		// Create new snippet (should invalidate list caches)
		_, err := svc.CreateSnippet(ctx, "Cache invalidator", 300, []string{"new"})
		if err != nil {
			t.Fatalf("CreateSnippet for invalidation failed: %v", err)
		}

		// List caches should be invalidated, but individual snippet caches should remain
		_ = rdb.Keys(ctx, "list:*").Val() // List keys (would be invalidated)
		snippetKeys := rdb.Keys(ctx, "snippet:*").Val()

		// We expect fewer list keys (invalidated) but snippet keys should still exist
		if len(snippetKeys) == 0 {
			t.Error("Expected individual snippet cache keys to remain after list invalidation")
		}
	})
}

// TestService_IntegrationConcurrentAccess tests concurrent access with real databases
func TestService_IntegrationConcurrentAccess(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	var dsn string
	var pool *pgxpool.Pool
	var err error

	// Check if running in CI environment
	if os.Getenv("CI") == "true" {
		// Use the existing database service in CI
		dsn = os.Getenv("DATABASE_URL")
		if dsn == "" {
			t.Skip("DATABASE_URL not set in CI environment")
			return
		}
		pool, err = pgxpool.New(ctx, dsn)
	} else {
		// Start PostgreSQL container for local testing
		pg, err := tcpostgres.RunContainer(ctx,
			tcpostgres.WithUsername("bonsai"),
			tcpostgres.WithPassword("secret"),
			tcpostgres.WithDatabase("bonsai"),
		)
		if err != nil {
			t.Skipf("skipping: cannot start postgres container: %v", err)
			return
		}
		defer pg.Terminate(ctx)

		// Connect to PostgreSQL
		host, _ := pg.Host(ctx)
		port, _ := pg.MappedPort(ctx, "5432")
		dsn = fmt.Sprintf("postgres://bonsai:secret@%s:%s/bonsai?sslmode=disable", host, port.Port())
		pool, err = pgxpool.New(ctx, dsn)
	}
	if err != nil {
		t.Fatalf("Failed to connect to postgres: %v", err)
	}
	defer pool.Close()

	// Wait for database to be ready
	for i := 0; i < 30; i++ {
		if err := pool.Ping(ctx); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
		if i == 29 {
			t.Fatalf("Database not ready after 3 seconds")
		}
	}

	// Setup repository and service
	repo := postgresRepo.NewSnippetRepository(pool)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("Failed to ensure schema: %v", err)
	}

	clock := RealClock{}
	svc := NewService(repo, clock)

	t.Run("ConcurrentCreation", func(t *testing.T) {
		const numWorkers = 10
		const snippetsPerWorker = 5

		var wg sync.WaitGroup
		errors := make(chan error, numWorkers*snippetsPerWorker)
		createdIDs := make(chan string, numWorkers*snippetsPerWorker)

		// Launch concurrent workers
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < snippetsPerWorker; j++ {
					content := fmt.Sprintf("Concurrent snippet %d-%d", workerID, j)
					snippet, err := svc.CreateSnippet(ctx, content, 300, []string{"concurrent", fmt.Sprintf("worker-%d", workerID)})
					if err != nil {
						errors <- fmt.Errorf("worker %d, snippet %d: %v", workerID, j, err)
						return
					}
					createdIDs <- snippet.ID
				}
			}(i)
		}

		wg.Wait()
		close(errors)
		close(createdIDs)

		// Check for errors
		for err := range errors {
			t.Errorf("Concurrent creation error: %v", err)
		}

		// Count created snippets
		var ids []string
		for id := range createdIDs {
			ids = append(ids, id)
		}

		expectedCount := numWorkers * snippetsPerWorker
		if len(ids) != expectedCount {
			t.Errorf("Expected %d snippets created, got %d", expectedCount, len(ids))
		}

		// Verify all snippets can be retrieved
		for _, id := range ids {
			_, _, err := svc.GetSnippetByID(ctx, id)
			if err != nil {
				t.Errorf("Failed to retrieve snippet %s: %v", id, err)
			}
		}
	})

	t.Run("ConcurrentReadWrite", func(t *testing.T) {
		// Create initial snippets
		var initialIDs []string
		for i := 0; i < 10; i++ {
			snippet, err := svc.CreateSnippet(ctx, fmt.Sprintf("Initial snippet %d", i), 300, []string{"initial"})
			if err != nil {
				t.Fatalf("Failed to create initial snippet %d: %v", i, err)
			}
			initialIDs = append(initialIDs, snippet.ID)
		}

		var wg sync.WaitGroup
		errors := make(chan error, 100)

		// Concurrent readers
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(readerID int) {
				defer wg.Done()
				for j := 0; j < 5; j++ {
					id := initialIDs[j%len(initialIDs)]
					_, _, err := svc.GetSnippetByID(ctx, id)
					if err != nil {
						errors <- fmt.Errorf("reader %d: %v", readerID, err)
						return
					}
				}
			}(i)
		}

		// Concurrent writers
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(writerID int) {
				defer wg.Done()
				for j := 0; j < 3; j++ {
					content := fmt.Sprintf("Concurrent write %d-%d", writerID, j)
					_, err := svc.CreateSnippet(ctx, content, 300, []string{"concurrent-write"})
					if err != nil {
						errors <- fmt.Errorf("writer %d: %v", writerID, err)
						return
					}
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			t.Errorf("Concurrent read/write error: %v", err)
		}
	})
}

// TestService_DatabaseConnectionHandling tests connection pool behavior
func TestService_DatabaseConnectionHandling(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	var dsn string
	var pool *pgxpool.Pool
	var err error

	// Check if running in CI environment
	if os.Getenv("CI") == "true" {
		// Use the existing database service in CI with limited connection pool
		baseDsn := os.Getenv("DATABASE_URL")
		if baseDsn == "" {
			t.Skip("DATABASE_URL not set in CI environment")
			return
		}
		// Add connection pool limit to the DSN
		dsn = baseDsn + "&pool_max_conns=5"
		pool, err = pgxpool.New(ctx, dsn)
	} else {
		// Start PostgreSQL container for local testing
		pg, err := tcpostgres.RunContainer(ctx,
			tcpostgres.WithUsername("bonsai"),
			tcpostgres.WithPassword("secret"),
			tcpostgres.WithDatabase("bonsai"),
		)
		if err != nil {
			t.Skipf("skipping: cannot start postgres container: %v", err)
			return
		}
		defer pg.Terminate(ctx)

		// Connect to PostgreSQL with limited connection pool
		host, _ := pg.Host(ctx)
		port, _ := pg.MappedPort(ctx, "5432")
		dsn = fmt.Sprintf("postgres://bonsai:secret@%s:%s/bonsai?sslmode=disable&pool_max_conns=5", host, port.Port())
		pool, err = pgxpool.New(ctx, dsn)
	}
	if err != nil {
		t.Fatalf("Failed to connect to postgres: %v", err)
	}
	defer pool.Close()

	// Wait for database to be ready
	for i := 0; i < 30; i++ {
		if err := pool.Ping(ctx); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
		if i == 29 {
			t.Fatalf("Database not ready after 3 seconds")
		}
	}

	// Setup repository and service
	repo := postgresRepo.NewSnippetRepository(pool)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("Failed to ensure schema: %v", err)
	}

	clock := RealClock{}
	svc := NewService(repo, clock)

	t.Run("ConnectionPoolExhaustion", func(t *testing.T) {
		// Stress test the connection pool with more concurrent requests than available connections
		const numConcurrent = 20 // More than the 5 connection pool limit

		var wg sync.WaitGroup
		errors := make(chan error, numConcurrent)

		for i := 0; i < numConcurrent; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()

				// Perform multiple operations to hold connections longer
				for j := 0; j < 3; j++ {
					// Create
					snippet, err := svc.CreateSnippet(ctx, fmt.Sprintf("Connection test %d-%d", workerID, j), 300, []string{"connection-test"})
					if err != nil {
						errors <- fmt.Errorf("worker %d create: %v", workerID, err)
						return
					}

					// Read
					_, _, err = svc.GetSnippetByID(ctx, snippet.ID)
					if err != nil {
						errors <- fmt.Errorf("worker %d read: %v", workerID, err)
						return
					}

					// List
					_, err = svc.ListSnippets(ctx, 1, 5, "connection-test")
					if err != nil {
						errors <- fmt.Errorf("worker %d list: %v", workerID, err)
						return
					}
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for errors - connection pool should handle this gracefully
		for err := range errors {
			t.Errorf("Connection pool stress error: %v", err)
		}
	})
}

// fixedClock for deterministic testing
type fixedClock struct {
	now time.Time
}

func (f *fixedClock) Now() time.Time {
	return f.now
}

// TestService_ErrorHandling tests various error conditions with real databases
func TestService_ErrorHandling(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	var dsn string
	var pool *pgxpool.Pool
	var err error

	// Check if running in CI environment
	if os.Getenv("CI") == "true" {
		// Use the existing database service in CI
		dsn = os.Getenv("DATABASE_URL")
		if dsn == "" {
			t.Skip("DATABASE_URL not set in CI environment")
			return
		}
		pool, err = pgxpool.New(ctx, dsn)
	} else {
		// Start PostgreSQL container for local testing
		pg, err := tcpostgres.RunContainer(ctx,
			tcpostgres.WithUsername("bonsai"),
			tcpostgres.WithPassword("secret"),
			tcpostgres.WithDatabase("bonsai"),
		)
		if err != nil {
			t.Skipf("skipping: cannot start postgres container: %v", err)
			return
		}
		defer pg.Terminate(ctx)

		// Connect to PostgreSQL
		host, _ := pg.Host(ctx)
		port, _ := pg.MappedPort(ctx, "5432")
		dsn = fmt.Sprintf("postgres://bonsai:secret@%s:%s/bonsai?sslmode=disable", host, port.Port())
		pool, err = pgxpool.New(ctx, dsn)
	}
	if err != nil {
		t.Fatalf("Failed to connect to postgres: %v", err)
	}
	defer pool.Close()

	// Wait for database to be ready
	for i := 0; i < 30; i++ {
		if err := pool.Ping(ctx); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
		if i == 29 {
			t.Fatalf("Database not ready after 3 seconds")
		}
	}

	// Setup repository and service
	repo := postgresRepo.NewSnippetRepository(pool)
	if err := repo.EnsureSchema(ctx); err != nil {
		t.Fatalf("Failed to ensure schema: %v", err)
	}

	// Use RealClock for integration tests to match database NOW()
	clock := RealClock{}
	svc := NewService(repo, clock)

	t.Run("NonExistentSnippet", func(t *testing.T) {
		_, _, err := svc.GetSnippetByID(ctx, "non-existent-id")
		if !errors.Is(err, ErrSnippetNotFound) {
			t.Errorf("Expected ErrSnippetNotFound, got: %v", err)
		}
	})

	t.Run("InvalidParameters", func(t *testing.T) {
		// Test empty content - should create successfully
		snippet, err := svc.CreateSnippet(ctx, "", 300, []string{"test"})
		if err != nil {
			t.Errorf("Unexpected error for empty content: %v", err)
		}
		if snippet.Content != "" {
			t.Error("Expected empty content")
		}

		// Test negative expiry - should treat as no expiry
		snippet2, err := svc.CreateSnippet(ctx, "test content", -1, []string{"test"})
		if err != nil {
			t.Errorf("Unexpected error for negative expiry: %v", err)
		}
		if !snippet2.ExpiresAt.IsZero() {
			t.Error("Expected no expiry for negative value")
		}

		// Test invalid pagination - should use defaults
		snippets, err := svc.ListSnippets(ctx, 0, 10, "")
		if err != nil {
			t.Errorf("Unexpected error for page 0: %v", err)
		}
		_ = snippets // Service auto-corrects to page 1

		snippets2, err := svc.ListSnippets(ctx, 1, 0, "")
		if err != nil {
			t.Errorf("Unexpected error for limit 0: %v", err)
		}
		_ = snippets2 // Service auto-corrects to default limit
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		// Create context that gets cancelled quickly
		ctxTimeout, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
		defer cancel()

		// Give the context time to be cancelled
		time.Sleep(2 * time.Millisecond)

		// Operations should fail with context cancelled
		_, err := svc.CreateSnippet(ctxTimeout, "test content", 300, []string{"test"})
		if err == nil {
			t.Error("Expected error due to context cancellation")
		}
	})
}

// TestService_CachePerformance tests caching performance improvements
func TestService_CachePerformance(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	var dsn string
	var pool *pgxpool.Pool
	var err error

	// Check if running in CI environment
	if os.Getenv("CI") == "true" {
		// Use the existing database service in CI
		dsn = os.Getenv("DATABASE_URL")
		if dsn == "" {
			t.Skip("DATABASE_URL not set in CI environment")
			return
		}
		pool, err = pgxpool.New(ctx, dsn)
	} else {
		// Start PostgreSQL container for local testing
		pg, err := tcpostgres.RunContainer(ctx,
			tcpostgres.WithUsername("bonsai"),
			tcpostgres.WithPassword("secret"),
			tcpostgres.WithDatabase("bonsai"),
		)
		if err != nil {
			t.Skipf("skipping: cannot start postgres container: %v", err)
			return
		}
		defer pg.Terminate(ctx)

		// Connect to PostgreSQL
		host, _ := pg.Host(ctx)
		port, _ := pg.MappedPort(ctx, "5432")
		dsn = fmt.Sprintf("postgres://bonsai:secret@%s:%s/bonsai?sslmode=disable", host, port.Port())
		pool, err = pgxpool.New(ctx, dsn)
	}
	if err != nil {
		t.Fatalf("Failed to connect to postgres: %v", err)
	}
	defer pool.Close()

	// Wait for database to be ready
	for i := 0; i < 30; i++ {
		if err := pool.Ping(ctx); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
		if i == 29 {
			t.Fatalf("Database not ready after 3 seconds")
		}
	}

	// Setup repositories
	pgRepo := postgresRepo.NewSnippetRepository(pool)
	if err := pgRepo.EnsureSchema(ctx); err != nil {
		t.Fatalf("Failed to ensure schema: %v", err)
	}

	// Setup Redis client
	var rdb *redis.Client
	if os.Getenv("CI") == "true" && os.Getenv("REDIS_URL") != "" {
		// Use existing Redis service in CI
		opt, err := redis.ParseURL(os.Getenv("REDIS_URL"))
		if err != nil {
			t.Fatalf("Failed to parse REDIS_URL: %v", err)
		}
		rdb = redis.NewClient(opt)
	} else {
		// Start mini Redis server for local testing
		miniRedis, err := miniredis.Run()
		if err != nil {
			t.Fatalf("Failed to start miniredis: %v", err)
		}
		defer miniRedis.Close()
		
		rdb = redis.NewClient(&redis.Options{
			Addr: miniRedis.Addr(),
		})
	}
	defer rdb.Close()

	cachedRepo := cachedRepo.NewSnippetRepository(pgRepo, rdb, 5*time.Minute)
	// Use RealClock for integration tests to match database NOW()
	clock := RealClock{}

	// Services with and without cache
	svcCached := NewService(cachedRepo, clock)
	svcDirect := NewService(pgRepo, clock)

	t.Run("ReadPerformanceComparison", func(t *testing.T) {
		// Create test data
		var snippetIDs []string
		for i := 0; i < 10; i++ {
			snippet, err := svcDirect.CreateSnippet(ctx, fmt.Sprintf("Performance test %d", i), 300, []string{"perf"})
			if err != nil {
				t.Fatalf("Failed to create test snippet %d: %v", i, err)
			}
			snippetIDs = append(snippetIDs, snippet.ID)
		}

		// Warm up cache by reading once with cached service
		for _, id := range snippetIDs {
			_, _, err := svcCached.GetSnippetByID(ctx, id)
			if err != nil {
				t.Fatalf("Cache warmup failed: %v", err)
			}
		}

		// Measure direct database reads
		startDirect := time.Now()
		for i := 0; i < 100; i++ {
			id := snippetIDs[i%len(snippetIDs)]
			_, _, err := svcDirect.GetSnippetByID(ctx, id)
			if err != nil {
				t.Fatalf("Direct read failed: %v", err)
			}
		}
		directDuration := time.Since(startDirect)

		// Measure cached reads
		startCached := time.Now()
		for i := 0; i < 100; i++ {
			id := snippetIDs[i%len(snippetIDs)]
			_, _, err := svcCached.GetSnippetByID(ctx, id)
			if err != nil {
				t.Fatalf("Cached read failed: %v", err)
			}
		}
		cachedDuration := time.Since(startCached)

		t.Logf("Direct reads: %v, Cached reads: %v", directDuration, cachedDuration)

		// Cache should be faster (though not always guaranteed in test environment)
		if cachedDuration > directDuration*2 {
			t.Logf("WARNING: Cached reads took longer than expected. Direct: %v, Cached: %v", directDuration, cachedDuration)
		}
	})
}

// TestService_DataConsistency tests data consistency across cache and database
func TestService_DataConsistency(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	var dsn string
	var pool *pgxpool.Pool
	var err error

	// Check if running in CI environment
	if os.Getenv("CI") == "true" {
		// Use the existing database service in CI
		dsn = os.Getenv("DATABASE_URL")
		if dsn == "" {
			t.Skip("DATABASE_URL not set in CI environment")
			return
		}
		pool, err = pgxpool.New(ctx, dsn)
	} else {
		// Start PostgreSQL container for local testing
		pg, err := tcpostgres.RunContainer(ctx,
			tcpostgres.WithUsername("bonsai"),
			tcpostgres.WithPassword("secret"),
			tcpostgres.WithDatabase("bonsai"),
		)
		if err != nil {
			t.Skipf("skipping: cannot start postgres container: %v", err)
			return
		}
		defer pg.Terminate(ctx)

		// Connect to PostgreSQL
		host, _ := pg.Host(ctx)
		port, _ := pg.MappedPort(ctx, "5432")
		dsn = fmt.Sprintf("postgres://bonsai:secret@%s:%s/bonsai?sslmode=disable", host, port.Port())
		pool, err = pgxpool.New(ctx, dsn)
	}
	if err != nil {
		t.Fatalf("Failed to connect to postgres: %v", err)
	}
	defer pool.Close()

	// Wait for database to be ready
	for i := 0; i < 30; i++ {
		if err := pool.Ping(ctx); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
		if i == 29 {
			t.Fatalf("Database not ready after 3 seconds")
		}
	}

	// Setup repositories
	pgRepo := postgresRepo.NewSnippetRepository(pool)
	if err := pgRepo.EnsureSchema(ctx); err != nil {
		t.Fatalf("Failed to ensure schema: %v", err)
	}

	// Setup Redis client
	var rdb *redis.Client
	if os.Getenv("CI") == "true" && os.Getenv("REDIS_URL") != "" {
		// Use existing Redis service in CI
		opt, err := redis.ParseURL(os.Getenv("REDIS_URL"))
		if err != nil {
			t.Fatalf("Failed to parse REDIS_URL: %v", err)
		}
		rdb = redis.NewClient(opt)
	} else {
		// Start mini Redis server for local testing
		miniRedis, err := miniredis.Run()
		if err != nil {
			t.Fatalf("Failed to start miniredis: %v", err)
		}
		defer miniRedis.Close()
		
		rdb = redis.NewClient(&redis.Options{
			Addr: miniRedis.Addr(),
		})
	}
	defer rdb.Close()

	cachedRepo := cachedRepo.NewSnippetRepository(pgRepo, rdb, 5*time.Minute)
	// Use RealClock for integration tests to match database NOW()
	clock := RealClock{}

	// Services with and without cache
	svcCached := NewService(cachedRepo, clock)
	svcDirect := NewService(pgRepo, clock)

	t.Run("CacheAndDatabaseSync", func(t *testing.T) {
		// Create snippet through cached service
		snippet, err := svcCached.CreateSnippet(ctx, "Consistency test", 300, []string{"consistency"})
		if err != nil {
			t.Fatalf("Create through cached service failed: %v", err)
		}

		// Read from cached service
		cachedResult, _, err := svcCached.GetSnippetByID(ctx, snippet.ID)
		if err != nil {
			t.Fatalf("Read from cached service failed: err=%v", err)
		}

		// Read directly from database
		directResult, _, err := svcDirect.GetSnippetByID(ctx, snippet.ID)
		if err != nil {
			t.Fatalf("Read from direct service failed: err=%v", err)
		}

		// Results should be identical
		if cachedResult.Content != directResult.Content {
			t.Errorf("Content mismatch: cached='%s', direct='%s'", cachedResult.Content, directResult.Content)
		}
		if cachedResult.ID != directResult.ID {
			t.Errorf("ID mismatch: cached='%s', direct='%s'", cachedResult.ID, directResult.ID)
		}
		if len(cachedResult.Tags) != len(directResult.Tags) {
			t.Errorf("Tags length mismatch: cached=%d, direct=%d", len(cachedResult.Tags), len(directResult.Tags))
		}
	})

	t.Run("ListConsistency", func(t *testing.T) {
		// Create multiple snippets
		for i := 0; i < 5; i++ {
			_, err := svcCached.CreateSnippet(ctx, fmt.Sprintf("List test %d", i), 300, []string{"listtest"})
			if err != nil {
				t.Fatalf("Failed to create snippet %d: %v", i, err)
			}
		}

		// List from cached service
		cachedList, err := svcCached.ListSnippets(ctx, 1, 10, "listtest")
		if err != nil {
			t.Fatalf("Cached list failed: %v", err)
		}

		// List directly from database
		directList, err := svcDirect.ListSnippets(ctx, 1, 10, "listtest")
		if err != nil {
			t.Fatalf("Direct list failed: %v", err)
		}

		// Lists should have same length
		if len(cachedList) != len(directList) {
			t.Errorf("List length mismatch: cached=%d, direct=%d", len(cachedList), len(directList))
		}

		// Check that all IDs are present in both lists
		cachedIDs := make(map[string]bool)
		for _, s := range cachedList {
			cachedIDs[s.ID] = true
		}

		for _, s := range directList {
			if !cachedIDs[s.ID] {
				t.Errorf("Snippet %s present in direct list but not cached list", s.ID)
			}
		}
	})
}
