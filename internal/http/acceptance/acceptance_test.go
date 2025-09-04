//go:build acceptance || !integration
// +build acceptance !integration

package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/roguepikachu/bonsai/internal/domain"
	httpHandlers "github.com/roguepikachu/bonsai/internal/http/handler"
	appRouter "github.com/roguepikachu/bonsai/internal/http/router"
	cachedRepo "github.com/roguepikachu/bonsai/internal/repository/cached"
	postgresRepo "github.com/roguepikachu/bonsai/internal/repository/postgres"
	"github.com/roguepikachu/bonsai/internal/service"
)

const (
	updatedContent = "Updated content"
)

const ciTrue = "true"

var (
	testDatabaseURL = getEnvOrDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/bonsai_test?sslmode=disable")
	testRedisAddr   string
)

func init() {
	// Parse Redis address from URL
	redisURL := getEnvOrDefault("REDIS_URL", "redis://localhost:6379/1")
	if strings.HasPrefix(redisURL, "redis://") {
		addr := strings.TrimPrefix(redisURL, "redis://")
		if idx := strings.Index(addr, "/"); idx > 0 {
			testRedisAddr = addr[:idx]
		} else {
			testRedisAddr = addr
		}
	} else {
		testRedisAddr = "localhost:6379"
	}
	// Handle CI environment with different Redis port
	if os.Getenv("CI") == ciTrue && os.Getenv("REDIS_PORT") != "" {
		testRedisAddr = "localhost:" + os.Getenv("REDIS_PORT")
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

var (
	testServer *http.Server
	baseURL    string
	client     = &http.Client{Timeout: 10 * time.Second}
)

// TestMain orchestrates setup/teardown for E2E tests
func TestMain(m *testing.M) {
	// Parse flags to check for -short
	flag.Parse()

	// Skip acceptance tests when running with -short flag (unit tests)
	if testing.Short() {
		fmt.Println("Skipping acceptance tests in short mode")
		os.Exit(0)
	}

	// Start services (skip in CI as they're provided by GitHub Actions)
	if os.Getenv("CI") != ciTrue {
		if err := startServices(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to start services: %v\n", err)
			os.Exit(1)
		}
	}

	// Wait for services to be ready
	if err := waitForServices(); err != nil {
		fmt.Fprintf(os.Stderr, "Services not ready: %v\n", err)
		if os.Getenv("CI") != ciTrue {
			stopServices()
		}
		os.Exit(1)
	}

	// Start test server
	if err := startTestServer(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start test server: %v\n", err)
		if os.Getenv("CI") != ciTrue {
			stopServices()
		}
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	stopTestServer()
	if os.Getenv("CI") != ciTrue {
		stopServices()
	}

	os.Exit(code)
}

func startServices() error {
	cmd := exec.Command("make", "services")
	cmd.Dir = "../../../" // Set working directory to project root
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start services: %v\nOutput: %s\n", err, out)
	}
	return err
}

func stopServices() {
	cmd := exec.Command("make", "services-stop")
	cmd.Dir = "../../../" // Set working directory to project root
	_ = cmd.Run()         // Ignore error, best effort cleanup
}

func waitForServices() error {
	// Wait for PostgreSQL
	for i := 0; i < 60; i++ { // Increase timeout to 60 seconds
		pool, err := pgxpool.New(context.Background(), testDatabaseURL)
		if err == nil {
			if err := pool.Ping(context.Background()); err == nil {
				pool.Close()
				break
			}
			pool.Close()
		}
		time.Sleep(time.Second)
		if i == 59 {
			return fmt.Errorf("PostgreSQL not ready after 60 seconds")
		}
	}

	// Wait for Redis
	for i := 0; i < 30; i++ {
		rdb := redis.NewClient(&redis.Options{Addr: testRedisAddr, DB: 1})
		if err := rdb.Ping(context.Background()).Err(); err == nil {
			_ = rdb.Close() // Best effort cleanup
			break
		}
		_ = rdb.Close() // Best effort cleanup
		time.Sleep(time.Second)
		if i == 29 {
			return fmt.Errorf("Redis not ready after 30 seconds")
		}
	}

	return nil
}

func startTestServer() error {
	// Setup database connection
	pool, err := pgxpool.New(context.Background(), testDatabaseURL)
	if err != nil {
		return fmt.Errorf("create pg pool: %w", err)
	}

	// Create test database if it doesn't exist
	if err := createTestDatabase(); err != nil {
		return fmt.Errorf("create test database: %w", err)
	}

	// Setup repositories
	pgRepo := postgresRepo.NewSnippetRepository(pool)
	if err := pgRepo.EnsureSchema(context.Background()); err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}

	// Setup Redis client
	rdb := redis.NewClient(&redis.Options{Addr: testRedisAddr, DB: 1})
	cachedSnippetRepo := cachedRepo.NewSnippetRepository(pgRepo, rdb, 5*time.Minute)

	// Setup service
	svc := service.NewService(cachedSnippetRepo, service.RealClock{})

	// Setup handlers
	snippetHandler := httpHandlers.NewHandler(svc)
	healthHandler := httpHandlers.NewHealthHandler(pool, rdb)

	// Setup router
	router := appRouter.NewRouter(snippetHandler, healthHandler)

	// Start server
	testServer = &http.Server{
		Addr:    ":8081",
		Handler: router,
	}

	go func() {
		if err := testServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Test server error: %v\n", err)
		}
	}()

	baseURL = "http://localhost:8081"

	// Wait for server to be ready
	for i := 0; i < 10; i++ {
		resp, err := client.Get(baseURL + "/v1/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close() // Best effort cleanup
			return nil
		}
		if resp != nil {
			_ = resp.Body.Close() // Best effort cleanup
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("test server not ready")
}

func createTestDatabase() error {
	// Connect to default postgres database to create test database
	// Extract connection info from testDatabaseURL and connect to postgres database
	defaultURL := strings.Replace(testDatabaseURL, "/bonsai_test", "/postgres", 1)
	if os.Getenv("CI") == "true" {
		// In CI, use the bonsai database directly (already created by service container)
		defaultURL = strings.Replace(testDatabaseURL, "_test", "", 1)
	}
	pool, err := pgxpool.New(context.Background(), defaultURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Check if test database exists
	var exists bool
	err = pool.QueryRow(context.Background(), "SELECT EXISTS(SELECT datname FROM pg_catalog.pg_database WHERE datname = 'bonsai_test')").Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		_, err = pool.Exec(context.Background(), "CREATE DATABASE bonsai_test")
		if err != nil {
			return err
		}
	}

	return nil
}

func stopTestServer() {
	if testServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = testServer.Shutdown(ctx) // Best effort cleanup
	}
}

// Helper function to clean database between tests
func cleanDatabase(t *testing.T) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDatabaseURL)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	_, err = pool.Exec(context.Background(), "TRUNCATE TABLE snippets")
	if err != nil {
		t.Fatalf("Failed to clean database: %v", err)
	}

	// Clean Redis
	rdb := redis.NewClient(&redis.Options{Addr: testRedisAddr, DB: 1})
	defer func() { _ = rdb.Close() }() // Best effort cleanup
	rdb.FlushDB(context.Background())
}

// Helper to perform HTTP requests and decode JSON
func doJSONRequest(t *testing.T, method, url string, body any, v any) (int, http.Header) {
	t.Helper()
	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		buf = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if v != nil {
		dec := json.NewDecoder(resp.Body)
		if err := dec.Decode(v); err != nil {
			t.Fatalf("decode: %v", err)
		}
	} else {
		_, _ = io.Copy(io.Discard, resp.Body)
	}
	return resp.StatusCode, resp.Header
}

// Database verification helpers
func verifySnippetInDatabase(t *testing.T, id string, expected domain.Snippet) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDatabaseURL)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	var snippet domain.Snippet
	var tags []string
	var expiresAt *time.Time
	err = pool.QueryRow(context.Background(), `
		SELECT id, content, created_at, expires_at, tags 
		FROM snippets WHERE id = $1`, id).Scan(
		&snippet.ID, &snippet.Content, &snippet.CreatedAt, &expiresAt, &tags)
	if err != nil {
		t.Fatalf("Failed to query snippet from database: %v", err)
	}
	snippet.Tags = tags
	if expiresAt != nil {
		snippet.ExpiresAt = *expiresAt
	}

	if snippet.ID != expected.ID {
		t.Errorf("Database ID mismatch: got %s, want %s", snippet.ID, expected.ID)
	}
	if snippet.Content != expected.Content {
		t.Errorf("Database content mismatch: got %s, want %s", snippet.Content, expected.Content)
	}
	if len(snippet.Tags) != len(expected.Tags) {
		t.Errorf("Database tags length mismatch: got %d, want %d", len(snippet.Tags), len(expected.Tags))
	}
	for i, tag := range expected.Tags {
		if i < len(snippet.Tags) && snippet.Tags[i] != tag {
			t.Errorf("Database tag mismatch at index %d: got %s, want %s", i, snippet.Tags[i], tag)
		}
	}
}

func verifySnippetNotInDatabase(t *testing.T, id string) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDatabaseURL)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	var count int
	err = pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM snippets WHERE id = $1", id).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query snippet count: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected snippet %s to not exist in database, but found %d entries", id, count)
	}
}

func countSnippetsInDatabase(t *testing.T) int {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDatabaseURL)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	var count int
	err = pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM snippets").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count snippets: %v", err)
	}
	return count
}

func getSnippetsFromDatabase(t *testing.T, limit int) []domain.Snippet {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDatabaseURL)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	rows, err := pool.Query(context.Background(), `
		SELECT id, content, created_at, expires_at, tags 
		FROM snippets ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		t.Fatalf("Failed to query snippets: %v", err)
	}
	defer rows.Close()

	var snippets []domain.Snippet
	for rows.Next() {
		var snippet domain.Snippet
		var tags []string
		var expiresAt *time.Time
		err := rows.Scan(&snippet.ID, &snippet.Content, &snippet.CreatedAt, &expiresAt, &tags)
		if err != nil {
			t.Fatalf("Failed to scan snippet: %v", err)
		}
		snippet.Tags = tags
		if expiresAt != nil {
			snippet.ExpiresAt = *expiresAt
		}
		snippets = append(snippets, snippet)
	}
	return snippets
}

// Redis verification helpers
func verifySnippetInRedis(t *testing.T, id string) bool {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: testRedisAddr, DB: 1})
	defer func() { _ = rdb.Close() }() // Best effort cleanup

	key := fmt.Sprintf("snippet:%s", id)
	exists, err := rdb.Exists(context.Background(), key).Result()
	if err != nil {
		t.Fatalf("Failed to check Redis key existence: %v", err)
	}
	return exists > 0
}

func verifySnippetNotInRedis(t *testing.T, id string) {
	t.Helper()
	if verifySnippetInRedis(t, id) {
		t.Errorf("Expected snippet %s to not exist in Redis cache", id)
	}
}

func getRedisKeyCount(t *testing.T, pattern string) int {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: testRedisAddr, DB: 1})
	defer func() { _ = rdb.Close() }() // Best effort cleanup

	keys, err := rdb.Keys(context.Background(), pattern).Result()
	if err != nil {
		t.Fatalf("Failed to get Redis keys: %v", err)
	}
	return len(keys)
}

func Test_HealthEndpoints(t *testing.T) {
	// Test /v1/health
	var healthResp struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
		Msg  string         `json:"message"`
	}
	code, _ := doJSONRequest(t, http.MethodGet, baseURL+"/v1/health", nil, &healthResp)
	if code != http.StatusOK || healthResp.Code != 200 || healthResp.Data["ok"] != true {
		t.Errorf("/health failed: code=%d resp=%+v", code, healthResp)
	}

	// Test /v1/livez
	var liveResp struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	code, _ = doJSONRequest(t, http.MethodGet, baseURL+"/v1/livez", nil, &liveResp)
	if code != http.StatusOK || liveResp.Data["status"] != "alive" {
		t.Errorf("/livez failed: code=%d resp=%+v", code, liveResp)
	}

	// Test /v1/readyz
	var readyResp struct {
		Code int `json:"code"`
		Data struct {
			Ready  bool             `json:"ready"`
			Checks []map[string]any `json:"checks"`
		} `json:"data"`
	}
	code, _ = doJSONRequest(t, http.MethodGet, baseURL+"/v1/readyz", nil, &readyResp)
	if code != http.StatusOK || !readyResp.Data.Ready {
		t.Errorf("/readyz failed: code=%d resp=%+v", code, readyResp)
	}
}

func Test_SnippetCRUD(t *testing.T) {
	cleanDatabase(t)

	// Verify database is clean
	initialCount := countSnippetsInDatabase(t)
	if initialCount != 0 {
		t.Fatalf("Expected empty database, found %d snippets", initialCount)
	}

	// Verify Redis is clean
	initialRedisKeys := getRedisKeyCount(t, "snippet:*")
	if initialRedisKeys != 0 {
		t.Fatalf("Expected no Redis keys, found %d", initialRedisKeys)
	}

	// Test Create Snippet
	createReq := map[string]any{
		"content":    "Hello, World!",
		"expires_in": 300, // 5 minutes
		"tags":       []string{"test", "demo"},
	}
	var created struct {
		ID        string   `json:"id"`
		Content   string   `json:"content"`
		CreatedAt string   `json:"created_at"`
		ExpiresAt *string  `json:"expires_at"`
		Tags      []string `json:"tags"`
	}
	code, _ := doJSONRequest(t, http.MethodPost, baseURL+"/v1/snippets", createReq, &created)
	if code != http.StatusCreated {
		t.Fatalf("Create failed: expected 201, got %d", code)
	}
	if created.ID == "" || created.Content != "Hello, World!" {
		t.Errorf("Unexpected created snippet: %+v", created)
	}
	if len(created.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(created.Tags))
	}

	snippetID := created.ID

	// Verify snippet was saved to database
	expectedSnippet := domain.Snippet{
		ID:      snippetID,
		Content: "Hello, World!",
		Tags:    []string{"test", "demo"},
	}
	verifySnippetInDatabase(t, snippetID, expectedSnippet)

	// Verify database count increased
	if countSnippetsInDatabase(t) != 1 {
		t.Errorf("Expected 1 snippet in database after creation")
	}

	// Note: Snippet might already be cached depending on implementation details

	// Test Get Snippet by ID
	var retrieved struct {
		ID        string   `json:"id"`
		Content   string   `json:"content"`
		CreatedAt string   `json:"created_at"`
		ExpiresAt *string  `json:"expires_at"`
		Tags      []string `json:"tags"`
	}
	code, hdr := doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets/"+snippetID, nil, &retrieved)
	if code != http.StatusOK {
		t.Fatalf("Get failed: expected 200, got %d", code)
	}
	if retrieved.ID != snippetID || retrieved.Content != "Hello, World!" {
		t.Errorf("Retrieved snippet mismatch: %+v", retrieved)
	}
	if hdr.Get("X-Cache") == "" {
		t.Error("Expected X-Cache header")
	}

	// Verify snippet is now cached in Redis
	if !verifySnippetInRedis(t, snippetID) {
		t.Error("Expected snippet to be cached in Redis after first read")
	}

	// Second GET should hit Redis cache
	code, hdr2 := doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets/"+snippetID, nil, &retrieved)
	if code != http.StatusOK {
		t.Fatalf("Second get failed: expected 200, got %d", code)
	}
	if hdr2.Get("X-Cache") == "" {
		t.Error("Expected X-Cache header on cached request")
	}

	// Test List Snippets
	var list struct {
		Page  int `json:"page"`
		Limit int `json:"limit"`
		Items []struct {
			ID        string   `json:"id"`
			CreatedAt string   `json:"created_at"`
			ExpiresAt *string  `json:"expires_at"`
			Tags      []string `json:"tags"`
		} `json:"items"`
	}
	code, _ = doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets?page=1&limit=10", nil, &list)
	if code != http.StatusOK {
		t.Fatalf("List failed: expected 200, got %d", code)
	}
	if len(list.Items) != 1 || list.Items[0].ID != snippetID {
		t.Errorf("List mismatch: expected 1 item with ID %s, got %+v", snippetID, list.Items)
	}

	// Test Get Non-existent Snippet
	var errResp map[string]any
	code, _ = doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets/non-existent", nil, &errResp)
	if code != http.StatusNotFound {
		t.Errorf("Expected 404 for non-existent snippet, got %d", code)
	}

	// Verify non-existent snippet is not in database
	verifySnippetNotInDatabase(t, "non-existent")

	// Verify non-existent snippet is not in Redis
	verifySnippetNotInRedis(t, "non-existent")
}

func Test_SnippetUpdate(t *testing.T) {
	cleanDatabase(t)

	// Verify database is clean
	initialCount := countSnippetsInDatabase(t)
	if initialCount != 0 {
		t.Fatalf("Expected empty database, found %d snippets", initialCount)
	}

	// Create a snippet first
	createReq := map[string]any{
		"content":    "Original content",
		"expires_in": 300,
		"tags":       []string{"original", "test"},
	}
	var created struct {
		ID        string   `json:"id"`
		Content   string   `json:"content"`
		CreatedAt string   `json:"created_at"`
		ExpiresAt *string  `json:"expires_at"`
		Tags      []string `json:"tags"`
	}
	code, _ := doJSONRequest(t, http.MethodPost, baseURL+"/v1/snippets", createReq, &created)
	if code != http.StatusCreated {
		t.Fatalf("Create failed: expected 201, got %d", code)
	}

	snippetID := created.ID
	originalCreatedAt := created.CreatedAt

	// Test successful update
	updateReq := map[string]any{
		"content":    updatedContent,
		"expires_in": 600,
		"tags":       []string{"updated", "modified"},
	}
	var updated struct {
		ID        string   `json:"id"`
		Content   string   `json:"content"`
		CreatedAt string   `json:"created_at"`
		ExpiresAt *string  `json:"expires_at"`
		Tags      []string `json:"tags"`
	}
	code, _ = doJSONRequest(t, http.MethodPut, baseURL+"/v1/snippets/"+snippetID, updateReq, &updated)
	if code != http.StatusOK {
		t.Fatalf("Update failed: expected 200, got %d", code)
	}

	// Verify update response
	if updated.ID != snippetID {
		t.Errorf("ID should be preserved: expected %s, got %s", snippetID, updated.ID)
	}
	if updated.Content != updatedContent {
		t.Errorf("Content not updated: expected 'Updated content', got %s", updated.Content)
	}
	if updated.CreatedAt != originalCreatedAt {
		t.Errorf("CreatedAt should be preserved: expected %s, got %s", originalCreatedAt, updated.CreatedAt)
	}
	if len(updated.Tags) != 2 || updated.Tags[0] != "updated" || updated.Tags[1] != "modified" {
		t.Errorf("Tags not updated correctly: expected [updated modified], got %v", updated.Tags)
	}

	// Verify updated snippet in database
	expectedUpdated := domain.Snippet{
		ID:      snippetID,
		Content: updatedContent,
		Tags:    []string{"updated", "modified"},
	}
	verifySnippetInDatabase(t, snippetID, expectedUpdated)

	// Verify cached snippet is invalidated by reading again
	var retrieved struct {
		ID        string   `json:"id"`
		Content   string   `json:"content"`
		CreatedAt string   `json:"created_at"`
		ExpiresAt *string  `json:"expires_at"`
		Tags      []string `json:"tags"`
	}
	code, hdr := doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets/"+snippetID, nil, &retrieved)
	if code != http.StatusOK {
		t.Fatalf("Get after update failed: expected 200, got %d", code)
	}
	if retrieved.Content != updatedContent {
		t.Errorf("Retrieved content after update: expected 'Updated content', got %s", retrieved.Content)
	}
	if hdr.Get("X-Cache") == "" {
		t.Error("Expected X-Cache header after update")
	}

	// Test update non-existent snippet
	var errResp map[string]any
	code, _ = doJSONRequest(t, http.MethodPut, baseURL+"/v1/snippets/non-existent", updateReq, &errResp)
	if code != http.StatusNotFound {
		t.Errorf("Expected 404 for updating non-existent snippet, got %d", code)
	}

	// Test update with invalid JSON
	invalidJSON := `{"content":"test", invalid json}`
	req, _ := http.NewRequest(http.MethodPut, baseURL+"/v1/snippets/"+snippetID, bytes.NewBufferString(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid JSON, got %d", resp.StatusCode)
	}

	// Test update with empty content
	emptyContentReq := map[string]any{
		"content":    "",
		"expires_in": 300,
		"tags":       []string{"empty"},
	}
	code, _ = doJSONRequest(t, http.MethodPut, baseURL+"/v1/snippets/"+snippetID, emptyContentReq, &errResp)
	if code != http.StatusBadRequest {
		t.Errorf("Expected 400 for empty content, got %d", code)
	}

	// Test update with no expiry
	noExpiryReq := map[string]any{
		"content":    "No expiry content",
		"expires_in": 0,
		"tags":       []string{"no-expiry"},
	}
	var noExpiryUpdated struct {
		ID        string   `json:"id"`
		Content   string   `json:"content"`
		CreatedAt string   `json:"created_at"`
		ExpiresAt *string  `json:"expires_at"`
		Tags      []string `json:"tags"`
	}
	code, _ = doJSONRequest(t, http.MethodPut, baseURL+"/v1/snippets/"+snippetID, noExpiryReq, &noExpiryUpdated)
	if code != http.StatusOK {
		t.Fatalf("Update with no expiry failed: expected 200, got %d", code)
	}
	if noExpiryUpdated.ExpiresAt != nil {
		t.Errorf("Expected no expiry, got %v", *noExpiryUpdated.ExpiresAt)
	}

	// Verify database count remains 1
	if countSnippetsInDatabase(t) != 1 {
		t.Errorf("Expected 1 snippet in database after updates")
	}
}

func Test_SnippetValidation(t *testing.T) {
	cleanDatabase(t)

	testCases := []struct {
		name           string
		request        map[string]any
		expectedStatus int
	}{
		{
			name:           "Missing content",
			request:        map[string]any{"expires_in": 60},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Empty content",
			request:        map[string]any{"content": "", "expires_in": 60},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Content too large",
			request:        map[string]any{"content": strings.Repeat("a", 10241), "expires_in": 60},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Negative expires_in",
			request:        map[string]any{"content": "test", "expires_in": -1},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "expires_in too large (>30 days)",
			request:        map[string]any{"content": "test", "expires_in": 2592001},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Valid no expiry",
			request:        map[string]any{"content": "test", "expires_in": 0},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "Valid with tags",
			request:        map[string]any{"content": "test", "expires_in": 60, "tags": []string{"go", "test"}},
			expectedStatus: http.StatusCreated,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var resp map[string]any
			code, _ := doJSONRequest(t, http.MethodPost, baseURL+"/v1/snippets", tc.request, &resp)
			if code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d for %s", tc.expectedStatus, code, tc.name)
			}
		})
	}
}

func Test_ListPaginationAndFiltering(t *testing.T) {
	cleanDatabase(t)

	// Create test snippets
	snippets := []map[string]any{
		{"content": "Go code", "expires_in": 300, "tags": []string{"go", "programming"}},
		{"content": "Python code", "expires_in": 300, "tags": []string{"python", "programming"}},
		{"content": "Java code", "expires_in": 300, "tags": []string{"java", "programming"}},
		{"content": "JavaScript code", "expires_in": 300, "tags": []string{"javascript", "web"}},
		{"content": "HTML code", "expires_in": 300, "tags": []string{"html", "web"}},
	}

	for _, snippet := range snippets {
		var created struct {
			ID string `json:"id"`
		}
		code, _ := doJSONRequest(t, http.MethodPost, baseURL+"/v1/snippets", snippet, &created)
		if code != http.StatusCreated {
			t.Fatalf("Failed to create snippet: %d", code)
		}
		time.Sleep(10 * time.Millisecond) // Small delay to ensure different timestamps
	}

	// Test default pagination
	var list struct {
		Page  int `json:"page"`
		Limit int `json:"limit"`
		Items []struct {
			ID   string   `json:"id"`
			Tags []string `json:"tags"`
		} `json:"items"`
	}
	code, _ := doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets", nil, &list)
	if code != http.StatusOK {
		t.Fatalf("List failed: %d", code)
	}
	if list.Page != 1 || list.Limit != 20 || len(list.Items) != 5 {
		t.Errorf("Default pagination failed: page=%d, limit=%d, items=%d", list.Page, list.Limit, len(list.Items))
	}

	// Test custom pagination
	code, _ = doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets?page=1&limit=3", nil, &list)
	if code != http.StatusOK {
		t.Fatalf("Paginated list failed: %d", code)
	}
	if len(list.Items) != 3 {
		t.Errorf("Expected 3 items, got %d", len(list.Items))
	}

	code, _ = doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets?page=2&limit=3", nil, &list)
	if code != http.StatusOK {
		t.Fatalf("Second page failed: %d", code)
	}
	if len(list.Items) != 2 {
		t.Errorf("Expected 2 items on second page, got %d", len(list.Items))
	}

	// Test tag filtering
	code, _ = doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets?tag=programming", nil, &list)
	if code != http.StatusOK {
		t.Fatalf("Tag filter failed: %d", code)
	}
	if len(list.Items) != 3 {
		t.Errorf("Expected 3 items with 'programming' tag, got %d", len(list.Items))
	}

	code, _ = doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets?tag=web", nil, &list)
	if code != http.StatusOK {
		t.Fatalf("Tag filter failed: %d", code)
	}
	if len(list.Items) != 2 {
		t.Errorf("Expected 2 items with 'web' tag, got %d", len(list.Items))
	}

	// Test invalid pagination parameters
	var errResp map[string]any
	code, _ = doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets?page=0", nil, &errResp)
	if code != http.StatusBadRequest {
		t.Errorf("Expected 400 for page=0, got %d", code)
	}

	code, _ = doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets?limit=0", nil, &errResp)
	if code != http.StatusBadRequest {
		t.Errorf("Expected 400 for limit=0, got %d", code)
	}

	code, _ = doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets?limit=101", nil, &errResp)
	if code != http.StatusBadRequest {
		t.Errorf("Expected 400 for limit>100, got %d", code)
	}
}

func Test_CacheHeaders(t *testing.T) {
	cleanDatabase(t)

	// Create a snippet
	createReq := map[string]any{
		"content":    "Cache test",
		"expires_in": 300,
		"tags":       []string{"cache"},
	}
	var created struct {
		ID string `json:"id"`
	}
	code, _ := doJSONRequest(t, http.MethodPost, baseURL+"/v1/snippets", createReq, &created)
	if code != http.StatusCreated {
		t.Fatalf("Create failed: %d", code)
	}

	snippetID := created.ID

	// First request - should be from database
	var retrieved map[string]any
	code, hdr := doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets/"+snippetID, nil, &retrieved)
	if code != http.StatusOK {
		t.Fatalf("First get failed: %d", code)
	}

	// Should have cache header indicating miss or database hit
	cacheHeader := hdr.Get("X-Cache")
	if cacheHeader == "" {
		t.Error("Expected X-Cache header on first request")
	}

	// Second request - should be from cache (if caching is working)
	code, hdr2 := doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets/"+snippetID, nil, &retrieved)
	if code != http.StatusOK {
		t.Fatalf("Second get failed: %d", code)
	}

	cacheHeader2 := hdr2.Get("X-Cache")
	if cacheHeader2 == "" {
		t.Error("Expected X-Cache header on second request")
	}
}

func Test_HeaderPropagation(t *testing.T) {
	// Test that X-Request-ID and X-Client-ID are properly handled
	req, _ := http.NewRequest(http.MethodGet, baseURL+"/v1/health", nil)
	req.Header.Set("X-Request-ID", "test-request-123")
	req.Header.Set("X-Client-ID", "test-client-456")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }() // Best effort cleanup

	if resp.Header.Get("X-Request-ID") != "test-request-123" {
		t.Errorf("Expected X-Request-ID to be echoed back")
	}
	if resp.Header.Get("X-Client-ID") != "test-client-456" {
		t.Errorf("Expected X-Client-ID to be echoed back")
	}

	// Test auto-generation when headers are missing
	req2, _ := http.NewRequest(http.MethodGet, baseURL+"/v1/health", nil)
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }() // Best effort cleanup

	if resp2.Header.Get("X-Request-ID") == "" {
		t.Error("Expected auto-generated X-Request-ID")
	}
	if resp2.Header.Get("X-Client-ID") == "" {
		t.Error("Expected auto-generated X-Client-ID")
	}
}

func Test_ExpiredSnippets(t *testing.T) {
	cleanDatabase(t)

	// Create a snippet with very short expiry
	createReq := map[string]any{
		"content":    "Short lived snippet",
		"expires_in": 1, // 1 second
		"tags":       []string{"temp"},
	}
	var created struct {
		ID string `json:"id"`
	}
	code, _ := doJSONRequest(t, http.MethodPost, baseURL+"/v1/snippets", createReq, &created)
	if code != http.StatusCreated {
		t.Fatalf("Create failed: %d", code)
	}

	snippetID := created.ID

	// Immediately retrieve - should work
	var retrieved map[string]any
	code, _ = doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets/"+snippetID, nil, &retrieved)
	if code != http.StatusOK {
		t.Errorf("Expected snippet to be accessible immediately: %d", code)
	}

	// Wait for expiry
	time.Sleep(2 * time.Second)

	// Try to retrieve expired snippet - should return 410 Gone
	var errResp map[string]any
	code, _ = doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets/"+snippetID, nil, &errResp)
	if code != http.StatusGone {
		t.Errorf("Expected 410 Gone for expired snippet, got %d", code)
	}

	// Verify expired snippet is still in database (not deleted)
	verifySnippetInDatabase(t, snippetID, domain.Snippet{
		ID:      snippetID,
		Content: "Short lived snippet",
		Tags:    []string{"temp"},
	})
}

func Test_CacheInvalidation(t *testing.T) {
	cleanDatabase(t)

	// Create multiple snippets
	snippetIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		createReq := map[string]any{
			"content":    fmt.Sprintf("Content %d", i+1),
			"expires_in": 300,
			"tags":       []string{fmt.Sprintf("tag%d", i+1)},
		}
		var created struct {
			ID string `json:"id"`
		}
		code, _ := doJSONRequest(t, http.MethodPost, baseURL+"/v1/snippets", createReq, &created)
		if code != http.StatusCreated {
			t.Fatalf("Create %d failed: %d", i+1, code)
		}
		snippetIDs[i] = created.ID
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Verify all snippets in database
	if countSnippetsInDatabase(t) != 3 {
		t.Errorf("Expected 3 snippets in database")
	}

	// Populate cache by reading all snippets
	for _, id := range snippetIDs {
		var retrieved map[string]any
		doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets/"+id, nil, &retrieved)
	}

	// Verify all snippets are cached
	for _, id := range snippetIDs {
		if !verifySnippetInRedis(t, id) {
			t.Errorf("Expected snippet %s to be cached", id)
		}
	}

	// Cache list queries
	var list map[string]any
	doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets?page=1&limit=10", nil, &list)

	// Verify list cache keys exist
	listCacheCount := getRedisKeyCount(t, "snippets:*")
	if listCacheCount == 0 {
		t.Error("Expected list cache keys to exist")
	}

	// Create a new snippet - should invalidate list caches
	createReq := map[string]any{
		"content":    "New Content",
		"expires_in": 300,
		"tags":       []string{"new"},
	}
	var newSnippet struct {
		ID string `json:"id"`
	}
	code, _ := doJSONRequest(t, http.MethodPost, baseURL+"/v1/snippets", createReq, &newSnippet)
	if code != http.StatusCreated {
		t.Fatalf("New snippet creation failed: %d", code)
	}

	// Verify database now has 4 snippets
	if countSnippetsInDatabase(t) != 4 {
		t.Errorf("Expected 4 snippets in database after new creation")
	}

	// Verify new snippet is in database
	verifySnippetInDatabase(t, newSnippet.ID, domain.Snippet{
		ID:      newSnippet.ID,
		Content: "New Content",
		Tags:    []string{"new"},
	})

	// List caches should be invalidated
	time.Sleep(100 * time.Millisecond) // Give cache invalidation time to process
	newListCacheCount := getRedisKeyCount(t, "snippets:*")
	if newListCacheCount >= listCacheCount {
		t.Errorf("Expected list caches to be invalidated, had %d, now have %d", listCacheCount, newListCacheCount)
	}
}

func Test_ConcurrentOperations(t *testing.T) {
	cleanDatabase(t)

	const numGoroutines = 10
	const snippetsPerGoroutine = 5

	var wg sync.WaitGroup
	snippetChan := make(chan string, numGoroutines*snippetsPerGoroutine)
	errorChan := make(chan error, numGoroutines)

	// Concurrent snippet creation
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < snippetsPerGoroutine; j++ {
				createReq := map[string]any{
					"content":    fmt.Sprintf("Concurrent content %d-%d", goroutineID, j),
					"expires_in": 300,
					"tags":       []string{fmt.Sprintf("concurrent-%d", goroutineID), "load-test"},
				}
				var created struct {
					ID string `json:"id"`
				}
				code, _ := doJSONRequest(t, http.MethodPost, baseURL+"/v1/snippets", createReq, &created)
				if code != http.StatusCreated {
					errorChan <- fmt.Errorf("goroutine %d, snippet %d failed with code %d", goroutineID, j, code)
					return
				}
				snippetChan <- created.ID
			}
		}(i)
	}

	wg.Wait()
	close(snippetChan)
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		t.Errorf("Concurrent creation error: %v", err)
	}

	// Collect all created snippet IDs
	var createdIDs []string
	for id := range snippetChan {
		createdIDs = append(createdIDs, id)
	}

	expectedCount := numGoroutines * snippetsPerGoroutine
	if len(createdIDs) != expectedCount {
		t.Fatalf("Expected %d snippets created, got %d", expectedCount, len(createdIDs))
	}

	// Verify all snippets in database
	dbCount := countSnippetsInDatabase(t)
	if dbCount != expectedCount {
		t.Errorf("Expected %d snippets in database, found %d", expectedCount, dbCount)
	}

	// Concurrent read operations
	wg = sync.WaitGroup{}
	readErrors := make(chan error, len(createdIDs))

	for _, id := range createdIDs {
		wg.Add(1)
		go func(snippetID string) {
			defer wg.Done()
			var retrieved map[string]any
			code, _ := doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets/"+snippetID, nil, &retrieved)
			if code != http.StatusOK {
				readErrors <- fmt.Errorf("failed to read snippet %s: code %d", snippetID, code)
			}
		}(id)
	}

	wg.Wait()
	close(readErrors)

	// Check for read errors
	for err := range readErrors {
		t.Errorf("Concurrent read error: %v", err)
	}

	// Verify some snippets got cached
	cachedCount := getRedisKeyCount(t, "snippet:*")
	if cachedCount == 0 {
		t.Error("Expected some snippets to be cached after concurrent reads")
	}

	t.Logf("Successfully processed %d concurrent snippets, %d cached", expectedCount, cachedCount)
}

func Test_DatabaseConsistency(t *testing.T) {
	cleanDatabase(t)

	// Create snippets with different expiry times
	testCases := []struct {
		content   string
		expiresIn int
		tags      []string
	}{
		{"No expiry content", 0, []string{"permanent"}},
		{"Short expiry content", 1, []string{"temporary"}},
		{"Long expiry content", 3600, []string{"long-lived", "important"}},
		{"Medium expiry content", 300, []string{"medium", "test"}},
	}

	createdSnippets := make([]struct {
		ID        string
		Content   string
		ExpiresIn int
		Tags      []string
	}, len(testCases))

	// Create all snippets
	for i, tc := range testCases {
		createReq := map[string]any{
			"content":    tc.content,
			"expires_in": tc.expiresIn,
			"tags":       tc.tags,
		}
		var created struct {
			ID        string   `json:"id"`
			Content   string   `json:"content"`
			ExpiresAt *string  `json:"expires_at"`
			Tags      []string `json:"tags"`
		}
		code, _ := doJSONRequest(t, http.MethodPost, baseURL+"/v1/snippets", createReq, &created)
		if code != http.StatusCreated {
			t.Fatalf("Failed to create snippet %d: %d", i, code)
		}

		createdSnippets[i] = struct {
			ID        string
			Content   string
			ExpiresIn int
			Tags      []string
		}{
			ID:        created.ID,
			Content:   created.Content,
			ExpiresIn: tc.expiresIn,
			Tags:      created.Tags,
		}

		// Verify immediate database consistency
		verifySnippetInDatabase(t, created.ID, domain.Snippet{
			ID:      created.ID,
			Content: created.Content,
			Tags:    created.Tags,
		})
	}

	// Verify total count in database
	if countSnippetsInDatabase(t) != len(testCases) {
		t.Errorf("Expected %d snippets in database", len(testCases))
	}

	// Test database queries directly
	dbSnippets := getSnippetsFromDatabase(t, 10)
	if len(dbSnippets) != len(testCases) {
		t.Errorf("Database query returned %d snippets, expected %d", len(dbSnippets), len(testCases))
	}

	// Verify ordering (should be DESC by created_at)
	for i := 1; i < len(dbSnippets); i++ {
		if dbSnippets[i].CreatedAt.After(dbSnippets[i-1].CreatedAt) {
			t.Errorf("Database ordering incorrect: snippet %d created after snippet %d", i, i-1)
		}
	}

	// Test tag filtering in API vs database
	var taggedList struct {
		Items []struct {
			ID   string   `json:"id"`
			Tags []string `json:"tags"`
		} `json:"items"`
	}
	code, _ := doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets?tag=test", nil, &taggedList)
	if code != http.StatusOK {
		t.Fatalf("Tag filter request failed: %d", code)
	}

	// Should only return snippets with "test" tag
	expectedTestSnippets := 1 // Only "Medium expiry content" has "test" tag
	if len(taggedList.Items) != expectedTestSnippets {
		t.Errorf("Expected %d snippets with 'test' tag, got %d", expectedTestSnippets, len(taggedList.Items))
	}

	// Wait for short expiry snippet to expire
	time.Sleep(2 * time.Second)

	// Test expired snippet behavior
	shortExpiryID := ""
	for _, s := range createdSnippets {
		if s.ExpiresIn == 1 {
			shortExpiryID = s.ID
			break
		}
	}

	if shortExpiryID != "" {
		var errResp map[string]any
		code, _ = doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets/"+shortExpiryID, nil, &errResp)
		if code != http.StatusGone {
			t.Errorf("Expected 410 Gone for expired snippet, got %d", code)
		}

		// Verify expired snippet still exists in database
		verifySnippetInDatabase(t, shortExpiryID, domain.Snippet{
			ID:      shortExpiryID,
			Content: "Short expiry content",
			Tags:    []string{"temporary"},
		})
	}
}

func Test_RedisFailover(t *testing.T) {
	cleanDatabase(t)

	// Create a snippet and cache it
	createReq := map[string]any{
		"content":    "Failover test content",
		"expires_in": 300,
		"tags":       []string{"failover"},
	}
	var created struct {
		ID string `json:"id"`
	}
	code, _ := doJSONRequest(t, http.MethodPost, baseURL+"/v1/snippets", createReq, &created)
	if code != http.StatusCreated {
		t.Fatalf("Create failed: %d", code)
	}

	snippetID := created.ID

	// Read to populate cache
	var retrieved map[string]any
	code, _ = doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets/"+snippetID, nil, &retrieved)
	if code != http.StatusOK {
		t.Fatalf("First read failed: %d", code)
	}

	// Verify it's cached
	if !verifySnippetInRedis(t, snippetID) {
		t.Fatal("Snippet should be cached")
	}

	// Manually clear Redis cache to simulate Redis failure/flush
	rdb := redis.NewClient(&redis.Options{Addr: testRedisAddr, DB: 1})
	defer func() { _ = rdb.Close() }() // Best effort cleanup
	rdb.FlushDB(context.Background())

	// Verify cache is cleared
	if verifySnippetInRedis(t, snippetID) {
		t.Error("Cache should be cleared")
	}

	// API should still work (fallback to database)
	code, _ = doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets/"+snippetID, nil, &retrieved)
	if code != http.StatusOK {
		t.Errorf("Read after cache clear failed: %d", code)
	}

	// Verify snippet is still in database
	verifySnippetInDatabase(t, snippetID, domain.Snippet{
		ID:      snippetID,
		Content: "Failover test content",
		Tags:    []string{"failover"},
	})

	// Snippet should be re-cached after read
	if !verifySnippetInRedis(t, snippetID) {
		t.Error("Snippet should be re-cached after database fallback")
	}
}

func Test_PaginationConsistency(t *testing.T) {
	cleanDatabase(t)

	// Create 25 snippets for pagination testing
	const totalSnippets = 25

	for i := 0; i < totalSnippets; i++ {
		createReq := map[string]any{
			"content":    fmt.Sprintf("Pagination content %02d", i+1),
			"expires_in": 3600,
			"tags":       []string{"pagination", fmt.Sprintf("page-test-%d", i%5)},
		}
		var created struct {
			ID string `json:"id"`
		}
		code, _ := doJSONRequest(t, http.MethodPost, baseURL+"/v1/snippets", createReq, &created)
		if code != http.StatusCreated {
			t.Fatalf("Failed to create snippet %d: %d", i+1, code)
		}
		time.Sleep(5 * time.Millisecond) // Ensure different timestamps
	}

	// Verify database count
	if countSnippetsInDatabase(t) != totalSnippets {
		t.Errorf("Expected %d snippets in database", totalSnippets)
	}

	// Test pagination consistency
	const pageSize = 10
	var allPaginatedIDs []string

	for page := 1; page <= 3; page++ {
		var list struct {
			Page  int `json:"page"`
			Limit int `json:"limit"`
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		}
		code, _ := doJSONRequest(t, http.MethodGet, fmt.Sprintf("%s/v1/snippets?page=%d&limit=%d", baseURL, page, pageSize), nil, &list)
		if code != http.StatusOK {
			t.Fatalf("Pagination page %d failed: %d", page, code)
		}

		expectedItemCount := pageSize
		if page == 3 {
			expectedItemCount = totalSnippets - (2 * pageSize) // Last page has remainder
		}

		if len(list.Items) != expectedItemCount {
			t.Errorf("Page %d: expected %d items, got %d", page, expectedItemCount, len(list.Items))
		}

		if list.Page != page || list.Limit != pageSize {
			t.Errorf("Page %d: incorrect metadata, got page=%d limit=%d", page, list.Page, list.Limit)
		}

		for _, item := range list.Items {
			allPaginatedIDs = append(allPaginatedIDs, item.ID)
		}
	}

	// Verify all snippets were returned across pages
	if len(allPaginatedIDs) != totalSnippets {
		t.Errorf("Pagination returned %d snippets, expected %d", len(allPaginatedIDs), totalSnippets)
	}

	// Verify no duplicates in pagination
	seenIDs := make(map[string]bool)
	for _, id := range allPaginatedIDs {
		if seenIDs[id] {
			t.Errorf("Duplicate ID %s found in pagination results", id)
		}
		seenIDs[id] = true
	}
}

func Test_PerformanceAndLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	cleanDatabase(t)

	// Performance test: Measure response times for various operations
	const numRequests = 100
	var createTimes, readTimes, listTimes []time.Duration

	// Test creation performance
	createdIDs := make([]string, 0, numRequests)
	for i := 0; i < numRequests; i++ {
		createReq := map[string]any{
			"content":    fmt.Sprintf("Performance test content %03d", i),
			"expires_in": 3600,
			"tags":       []string{"performance", "load-test", fmt.Sprintf("batch-%d", i/10)},
		}
		var created struct {
			ID string `json:"id"`
		}

		start := time.Now()
		code, _ := doJSONRequest(t, http.MethodPost, baseURL+"/v1/snippets", createReq, &created)
		createTime := time.Since(start)

		if code != http.StatusCreated {
			t.Errorf("Create request %d failed with code %d", i, code)
			continue
		}

		createdIDs = append(createdIDs, created.ID)
		createTimes = append(createTimes, createTime)
	}

	// Verify all snippets are in database
	dbCount := countSnippetsInDatabase(t)
	if dbCount != numRequests {
		t.Errorf("Expected %d snippets in database, found %d", numRequests, dbCount)
	}

	// Test read performance (mix of cache hits and misses)
	for i := 0; i < numRequests; i++ {
		// Use random snippet ID to test various cache scenarios
		snippetID := createdIDs[i%len(createdIDs)]
		var retrieved map[string]any

		start := time.Now()
		code, _ := doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets/"+snippetID, nil, &retrieved)
		readTime := time.Since(start)

		if code != http.StatusOK {
			t.Errorf("Read request %d failed with code %d", i, code)
			continue
		}

		readTimes = append(readTimes, readTime)
	}

	// Test list performance with various pagination parameters
	listParams := []string{
		"?page=1&limit=10",
		"?page=1&limit=20",
		"?page=5&limit=10",
		"?tag=performance",
		"?tag=load-test",
	}

	for i := 0; i < 20; i++ { // 4 requests per param type
		param := listParams[i%len(listParams)]
		var list map[string]any

		start := time.Now()
		code, _ := doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets"+param, nil, &list)
		listTime := time.Since(start)

		if code != http.StatusOK {
			t.Errorf("List request %d failed with code %d", i, code)
			continue
		}

		listTimes = append(listTimes, listTime)
	}

	// Calculate and report performance metrics
	avgCreateTime := calculateAverage(createTimes)
	p95CreateTime := calculatePercentile(createTimes, 95)
	maxCreateTime := calculateMax(createTimes)

	avgReadTime := calculateAverage(readTimes)
	p95ReadTime := calculatePercentile(readTimes, 95)
	maxReadTime := calculateMax(readTimes)

	avgListTime := calculateAverage(listTimes)
	p95ListTime := calculatePercentile(listTimes, 95)
	maxListTime := calculateMax(listTimes)

	// Log performance metrics
	t.Logf("CREATE Performance - Avg: %v, P95: %v, Max: %v", avgCreateTime, p95CreateTime, maxCreateTime)
	t.Logf("READ Performance - Avg: %v, P95: %v, Max: %v", avgReadTime, p95ReadTime, maxReadTime)
	t.Logf("LIST Performance - Avg: %v, P95: %v, Max: %v", avgListTime, p95ListTime, maxListTime)

	// Performance assertions (adjust thresholds based on your requirements)
	if avgCreateTime > 100*time.Millisecond {
		t.Errorf("Average create time too high: %v", avgCreateTime)
	}
	if avgReadTime > 50*time.Millisecond {
		t.Errorf("Average read time too high: %v", avgReadTime)
	}
	if avgListTime > 100*time.Millisecond {
		t.Errorf("Average list time too high: %v", avgListTime)
	}

	// Verify cache effectiveness
	cacheKeyCount := getRedisKeyCount(t, "snippet:*")
	if cacheKeyCount == 0 {
		t.Error("Expected some snippets to be cached")
	}

	cacheHitRatio := float64(cacheKeyCount) / float64(len(createdIDs))
	t.Logf("Cache hit ratio: %.2f%% (%d cached out of %d total)", cacheHitRatio*100, cacheKeyCount, len(createdIDs))
}

// Helper functions for performance calculations
func calculateAverage(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}

func calculatePercentile(durations []time.Duration, percentile int) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	// Simple percentile calculation (sort and pick index)
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)

	// Simple bubble sort for small datasets
	for i := 0; i < len(sorted); i++ {
		for j := 0; j < len(sorted)-1-i; j++ {
			if sorted[j] > sorted[j+1] {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	index := (len(sorted) * percentile) / 100
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func calculateMax(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	maxDuration := durations[0]
	for _, d := range durations {
		if d > maxDuration {
			maxDuration = d
		}
	}
	return maxDuration
}

// Test_SnippetUpdateEdgeCases tests edge cases for the PUT endpoint
func Test_SnippetUpdateEdgeCases(t *testing.T) {
	cleanDatabase(t)

	// Create a snippet first
	createReq := map[string]any{
		"content":    "Original content for update testing",
		"expires_in": 300,
		"tags":       []string{"original", "update-test"},
	}
	var created struct {
		ID        string    `json:"id"`
		CreatedAt time.Time `json:"created_at"`
	}
	code, _ := doJSONRequest(t, http.MethodPost, baseURL+"/v1/snippets", createReq, &created)
	if code != http.StatusCreated {
		t.Fatalf("Failed to create initial snippet: %d", code)
	}

	snippetID := created.ID
	originalCreatedAt := created.CreatedAt

	testCases := []struct {
		name           string
		request        map[string]any
		expectedStatus int
		validate       func(t *testing.T, resp map[string]any)
	}{
		{
			name:           "Valid update with all fields",
			request:        map[string]any{"content": updatedContent, "expires_in": 600, "tags": []string{"updated", "test"}},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp map[string]any) {
				if resp["content"].(string) != updatedContent {
					t.Errorf("Content not updated: %s", resp["content"])
				}
				if resp["tags"] != nil {
					if !equalStringArrays(resp["tags"].([]interface{}), []string{"updated", "test"}) {
						t.Errorf("Tags not updated: %v", resp["tags"])
					}
				} else {
					t.Errorf("Expected tags in response")
				}
			},
		},
		{
			name:           "Update with empty content",
			request:        map[string]any{"content": "", "expires_in": 300, "tags": []string{"empty"}},
			expectedStatus: http.StatusBadRequest, // Empty content not allowed by validation
			validate:       nil,
		},
		{
			name:           "Update with unicode content",
			request:        map[string]any{"content": "🚀 Hello 世界 مرحبا עולם 🌍", "expires_in": 300, "tags": []string{"unicode"}},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp map[string]any) {
				expected := "🚀 Hello 世界 مرحبا עולם 🌍"
				if resp["content"].(string) != expected {
					t.Errorf("Unicode content not preserved: got %s", resp["content"])
				}
			},
		},
		{
			name:           "Update with max content size",
			request:        map[string]any{"content": strings.Repeat("a", 10240), "expires_in": 300, "tags": []string{"large"}},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp map[string]any) {
				if len(resp["content"].(string)) != 10240 {
					t.Errorf("Large content length not preserved: %d", len(resp["content"].(string)))
				}
			},
		},
		{
			name:           "Update with over max content size",
			request:        map[string]any{"content": strings.Repeat("a", 10241), "expires_in": 300, "tags": []string{"toolarge"}},
			expectedStatus: http.StatusBadRequest,
			validate:       nil,
		},
		{
			name:           "Update with no expiration",
			request:        map[string]any{"content": "No expiry update", "expires_in": 0, "tags": []string{"permanent"}},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp map[string]any) {
				if resp["expires_at"] != nil {
					t.Errorf("Expected no expiration, got: %v", resp["expires_at"])
				}
			},
		},
		{
			name:           "Update with maximum expiration",
			request:        map[string]any{"content": "Max expiry update", "expires_in": 2592000, "tags": []string{"maxexp"}},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp map[string]any) {
				if resp["expires_at"] == nil {
					t.Error("Expected expiration to be set for max expiry")
				}
			},
		},
		{
			name:           "Update with over maximum expiration",
			request:        map[string]any{"content": "Over max expiry", "expires_in": 2592001, "tags": []string{"overmax"}},
			expectedStatus: http.StatusBadRequest,
			validate:       nil,
		},
		{
			name:           "Update with negative expiration",
			request:        map[string]any{"content": "Negative expiry", "expires_in": -1, "tags": []string{"negative"}},
			expectedStatus: http.StatusBadRequest,
			validate:       nil,
		},
		{
			name:           "Update with empty tags",
			request:        map[string]any{"content": "Empty tags update", "expires_in": 300, "tags": []string{}},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp map[string]any) {
				if resp["tags"] != nil {
					tags := resp["tags"].([]interface{})
					if len(tags) != 0 {
						t.Errorf("Expected empty tags, got: %v", tags)
					}
				}
			},
		},
		{
			name:           "Update with many tags",
			request:        map[string]any{"content": "Many tags update", "expires_in": 300, "tags": generateManyTags(20)},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp map[string]any) {
				if resp["tags"] != nil {
					tags := resp["tags"].([]interface{})
					if len(tags) != 20 {
						t.Errorf("Expected 20 tags, got: %d", len(tags))
					}
				} else {
					t.Errorf("Expected tags in response")
				}
			},
		},
		{
			name:           "Update with special character tags",
			request:        map[string]any{"content": "Special tags", "expires_in": 300, "tags": []string{"tag-with-dash", "tag_with_underscore", "tag.with.dots", "tag@symbol", "🚀emoji"}},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp map[string]any) {
				if resp["tags"] != nil {
					tags := resp["tags"].([]interface{})
					expected := []string{"tag-with-dash", "tag_with_underscore", "tag.with.dots", "tag@symbol", "🚀emoji"}
					if !equalStringArrays(tags, expected) {
						t.Errorf("Special character tags not preserved: %v", tags)
					}
				} else {
					t.Errorf("Expected tags in response")
				}
			},
		},
		{
			name:           "Missing content field",
			request:        map[string]any{"expires_in": 300, "tags": []string{"missing-content"}},
			expectedStatus: http.StatusBadRequest,
			validate:       nil,
		},
		{
			name:           "Update with whitespace content",
			request:        map[string]any{"content": "  \t\n\r  ", "expires_in": 300, "tags": []string{"whitespace"}},
			expectedStatus: http.StatusOK,
			validate: func(t *testing.T, resp map[string]any) {
				if resp["content"].(string) != "  \t\n\r  " {
					t.Errorf("Whitespace content not preserved: %q", resp["content"])
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var resp map[string]any
			code, _ := doJSONRequest(t, http.MethodPut, baseURL+"/v1/snippets/"+snippetID, tc.request, &resp)

			if code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, code)
				return
			}

			if tc.expectedStatus == http.StatusOK {
				// Verify CreatedAt is preserved for successful updates
				if createdAtStr, ok := resp["created_at"].(string); ok {
					respCreatedAt, err := time.Parse(time.RFC3339, createdAtStr)
					if err != nil {
						t.Errorf("Failed to parse created_at: %v", err)
					} else if !respCreatedAt.Equal(originalCreatedAt) {
						t.Errorf("CreatedAt not preserved: original=%v, response=%v", originalCreatedAt, respCreatedAt)
					}
				}

				// Verify ID is preserved
				if resp["id"].(string) != snippetID {
					t.Errorf("ID changed during update: original=%s, response=%s", snippetID, resp["id"])
				}

				if tc.validate != nil {
					tc.validate(t, resp)
				}
			}
		})
	}
}

func Test_UpdateNonExistentSnippet(t *testing.T) {
	cleanDatabase(t)

	nonExistentID := "non-existent-id-12345"
	updateReq := map[string]any{
		"content":    "Update non-existent",
		"expires_in": 300,
		"tags":       []string{"test"},
	}

	var resp map[string]any
	code, _ := doJSONRequest(t, http.MethodPut, baseURL+"/v1/snippets/"+nonExistentID, updateReq, &resp)

	if code != http.StatusNotFound {
		t.Errorf("Expected 404 for non-existent snippet, got %d", code)
	}
}

func Test_UpdateExpiredSnippet(t *testing.T) {
	cleanDatabase(t)

	// Create snippet with very short expiry
	createReq := map[string]any{
		"content":    "About to expire",
		"expires_in": 1, // 1 second
		"tags":       []string{"expiring"},
	}
	var created struct {
		ID string `json:"id"`
	}
	code, _ := doJSONRequest(t, http.MethodPost, baseURL+"/v1/snippets", createReq, &created)
	if code != http.StatusCreated {
		t.Fatalf("Failed to create snippet: %d", code)
	}

	snippetID := created.ID

	// Wait for expiry
	time.Sleep(2 * time.Second)

	// Try to update expired snippet
	updateReq := map[string]any{
		"content":    "Update expired snippet",
		"expires_in": 300,
		"tags":       []string{"updated"},
	}

	var resp map[string]any
	code, _ = doJSONRequest(t, http.MethodPut, baseURL+"/v1/snippets/"+snippetID, updateReq, &resp)

	if code != http.StatusGone {
		t.Errorf("Expected 410 Gone for expired snippet, got %d", code)
	}
}

func Test_UpdateCacheInvalidation(t *testing.T) {
	cleanDatabase(t)

	// Create and cache a snippet
	createReq := map[string]any{
		"content":    "Cache test content",
		"expires_in": 300,
		"tags":       []string{"cache", "test"},
	}
	var created struct {
		ID string `json:"id"`
	}
	code, _ := doJSONRequest(t, http.MethodPost, baseURL+"/v1/snippets", createReq, &created)
	if code != http.StatusCreated {
		t.Fatalf("Failed to create snippet: %d", code)
	}

	snippetID := created.ID

	// Read to populate cache
	var retrieved map[string]any
	doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets/"+snippetID, nil, &retrieved)

	// Verify it's cached
	if !verifySnippetInRedis(t, snippetID) {
		t.Error("Expected snippet to be cached")
	}

	// Update the snippet
	updateReq := map[string]any{
		"content":    "Updated cache content",
		"expires_in": 600,
		"tags":       []string{"updated", "cache"},
	}
	var updated map[string]any
	code, _ = doJSONRequest(t, http.MethodPut, baseURL+"/v1/snippets/"+snippetID, updateReq, &updated)
	if code != http.StatusOK {
		t.Fatalf("Update failed: %d", code)
	}

	// Cache should be invalidated after update
	if verifySnippetInRedis(t, snippetID) {
		t.Error("Expected cache to be invalidated after update")
	}

	// Read again to verify update and repopulate cache
	code, _ = doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets/"+snippetID, nil, &retrieved)
	if code != http.StatusOK {
		t.Fatalf("Read after update failed: %d", code)
	}

	// Verify content was updated
	if retrieved["content"].(string) != "Updated cache content" {
		t.Errorf("Content not updated: %s", retrieved["content"])
	}

	// Verify cache is repopulated
	if !verifySnippetInRedis(t, snippetID) {
		t.Error("Expected snippet to be re-cached after read")
	}
}

func Test_ConcurrentUpdates(t *testing.T) {
	cleanDatabase(t)

	// Create a snippet
	createReq := map[string]any{
		"content":    "Concurrent update test",
		"expires_in": 300,
		"tags":       []string{"concurrent"},
	}
	var created struct {
		ID string `json:"id"`
	}
	code, _ := doJSONRequest(t, http.MethodPost, baseURL+"/v1/snippets", createReq, &created)
	if code != http.StatusCreated {
		t.Fatalf("Failed to create snippet: %d", code)
	}

	snippetID := created.ID

	// Launch concurrent updates
	const numWorkers = 10
	var wg sync.WaitGroup
	results := make(chan struct {
		workerID int
		success  bool
		status   int
	}, numWorkers)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			updateReq := map[string]any{
				"content":    fmt.Sprintf("Updated by worker %d", workerID),
				"expires_in": 300 + workerID,
				"tags":       []string{fmt.Sprintf("worker-%d", workerID), "concurrent"},
			}

			var resp map[string]any
			code, _ := doJSONRequest(t, http.MethodPut, baseURL+"/v1/snippets/"+snippetID, updateReq, &resp)

			results <- struct {
				workerID int
				success  bool
				status   int
			}{
				workerID: workerID,
				success:  code == http.StatusOK,
				status:   code,
			}
		}(i)
	}

	wg.Wait()
	close(results)

	// Analyze results
	var successCount, errorCount int
	for result := range results {
		if result.success {
			successCount++
		} else {
			errorCount++
			t.Logf("Worker %d failed with status %d", result.workerID, result.status)
		}
	}

	if successCount == 0 {
		t.Error("No concurrent updates succeeded")
	}

	// Verify final state is consistent
	var final map[string]any
	code, _ = doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets/"+snippetID, nil, &final)
	if code != http.StatusOK {
		t.Fatalf("Failed to read final state: %d", code)
	}

	// Final content should be from one of the workers
	if final["content"].(string) == "Concurrent update test" {
		t.Error("Snippet was not updated by any worker")
	}

	t.Logf("Concurrent updates: %d succeeded, %d failed", successCount, errorCount)
}

func Test_UpdateMalformedRequests(t *testing.T) {
	cleanDatabase(t)

	// Create a snippet first
	createReq := map[string]any{
		"content":    "Test content for malformed updates",
		"expires_in": 300,
		"tags":       []string{"test"},
	}
	var created struct {
		ID string `json:"id"`
	}
	code, _ := doJSONRequest(t, http.MethodPost, baseURL+"/v1/snippets", createReq, &created)
	if code != http.StatusCreated {
		t.Fatalf("Failed to create snippet: %d", code)
	}

	snippetID := created.ID

	testCases := []struct {
		name        string
		contentType string
		body        string
		expected    int
	}{
		{
			name:        "Invalid JSON syntax",
			contentType: "application/json",
			body:        `{"content": "test", "expires_in": }`,
			expected:    http.StatusBadRequest,
		},
		{
			name:        "Wrong content type",
			contentType: "text/plain",
			body:        `content=test&expires_in=300`,
			expected:    http.StatusBadRequest,
		},
		{
			name:        "Empty body",
			contentType: "application/json",
			body:        ``,
			expected:    http.StatusBadRequest,
		},
		{
			name:        "Non-JSON body",
			contentType: "application/json",
			body:        `this is not json`,
			expected:    http.StatusBadRequest,
		},
		{
			name:        "Invalid field types",
			contentType: "application/json",
			body:        `{"content": 123, "expires_in": "not-a-number", "tags": "not-an-array"}`,
			expected:    http.StatusBadRequest,
		},
		{
			name:        "Null content",
			contentType: "application/json",
			body:        `{"content": null, "expires_in": 300, "tags": ["test"]}`,
			expected:    http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Timeout: 10 * time.Second}

			req, err := http.NewRequest(http.MethodPut, baseURL+"/v1/snippets/"+snippetID, strings.NewReader(tc.body))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			req.Header.Set("Content-Type", tc.contentType)

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != tc.expected {
				t.Errorf("Expected status %d, got %d", tc.expected, resp.StatusCode)
			}
		})
	}
}

func Test_UpdateIdempotency(t *testing.T) {
	cleanDatabase(t)

	// Create a snippet
	createReq := map[string]any{
		"content":    "Idempotency test",
		"expires_in": 300,
		"tags":       []string{"idempotent"},
	}
	var created struct {
		ID string `json:"id"`
	}
	code, _ := doJSONRequest(t, http.MethodPost, baseURL+"/v1/snippets", createReq, &created)
	if code != http.StatusCreated {
		t.Fatalf("Failed to create snippet: %d", code)
	}

	snippetID := created.ID
	updateReq := map[string]any{
		"content":    "Updated idempotent content",
		"expires_in": 600,
		"tags":       []string{"updated", "idempotent"},
	}

	// Perform the same update multiple times
	var responses []map[string]any
	for i := 0; i < 3; i++ {
		var resp map[string]any
		code, _ := doJSONRequest(t, http.MethodPut, baseURL+"/v1/snippets/"+snippetID, updateReq, &resp)
		if code != http.StatusOK {
			t.Fatalf("Update %d failed: %d", i+1, code)
		}
		responses = append(responses, resp)
	}

	// All responses should be identical (except timestamps might vary slightly)
	for i := 1; i < len(responses); i++ {
		if responses[i]["id"] != responses[0]["id"] {
			t.Errorf("ID differs between updates: %s vs %s", responses[0]["id"], responses[i]["id"])
		}
		if responses[i]["content"] != responses[0]["content"] {
			t.Errorf("Content differs between updates: %s vs %s", responses[0]["content"], responses[i]["content"])
		}
		if responses[i]["created_at"] != responses[0]["created_at"] {
			t.Errorf("CreatedAt differs between updates: %s vs %s", responses[0]["created_at"], responses[i]["created_at"])
		}
		// Note: expires_at might vary slightly due to timing, so we don't check it strictly
	}

	// Verify final state in database
	var final map[string]any
	code, _ = doJSONRequest(t, http.MethodGet, baseURL+"/v1/snippets/"+snippetID, nil, &final)
	if code != http.StatusOK {
		t.Fatalf("Failed to read final state: %d", code)
	}

	if final["content"] != "Updated idempotent content" {
		t.Errorf("Final content incorrect: %s", final["content"])
	}
}

// Helper function to generate many tags for testing
func generateManyTags(count int) []string {
	var tags []string
	for i := 0; i < count; i++ {
		tags = append(tags, fmt.Sprintf("tag%03d", i))
	}
	return tags
}

// Helper function to compare string arrays from JSON responses
func equalStringArrays(jsonArray []interface{}, expected []string) bool {
	if len(jsonArray) != len(expected) {
		return false
	}
	for i, item := range jsonArray {
		if str, ok := item.(string); !ok || str != expected[i] {
			return false
		}
	}
	return true
}
