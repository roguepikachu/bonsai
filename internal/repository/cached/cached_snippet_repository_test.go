//go:build integration

package cached

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/roguepikachu/bonsai/internal/domain"
	"github.com/roguepikachu/bonsai/internal/repository"
	"github.com/roguepikachu/bonsai/internal/repository/fake"
)

func TestCachedRepository_Roundtrip(t *testing.T) {
	ctx := context.Background()
	primary := fake.NewSnippetRepository()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rcli := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	repo := NewSnippetRepository(primary, rcli, time.Minute)

	now := time.Now().UTC()
	s := domain.Snippet{ID: "id1", Content: "hello", CreatedAt: now}
	if err := repo.Insert(ctx, s); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// cache should be populated; get should hit cache after first miss-fill
	got, err := repo.FindByID(ctx, "id1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.ID != "id1" {
		t.Fatalf("wrong id: %s", got.ID)
	}

	// list populates list cache
	lst, err := repo.List(ctx, 1, 10, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(lst) != 1 {
		t.Fatalf("want 1 item, got %d", len(lst))
	}

	// ensure snippet is stored in cache JSON
	k := keySnippet("id1")
	gotStr, gerr := rcli.Get(ctx, k).Result()
	if gerr != nil {
		t.Fatalf("cache get: %v", gerr)
	}
	var cached domain.Snippet
	if err := json.Unmarshal([]byte(gotStr), &cached); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cached.ID != "id1" {
		t.Fatalf("cache mismatch")
	}
}

func TestCachedRepository_CacheHit(t *testing.T) {
	ctx := context.Background()
	primary := fake.NewSnippetRepository()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rcli := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	repo := NewSnippetRepository(primary, rcli, time.Minute)

	now := time.Now().UTC()
	s := domain.Snippet{ID: "cached", Content: "cached content", CreatedAt: now, Tags: []string{"cache"}}
	if err := repo.Insert(ctx, s); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// First call should cache miss and fill
	_, err = repo.FindByID(ctx, "cached")
	if err != nil {
		t.Fatalf("first find: %v", err)
	}

	// Remove from primary to prove cache hit
	primary.DeleteByID("cached")

	// Second call should be cache hit
	got, err := repo.FindByID(ctx, "cached")
	if err != nil {
		t.Fatalf("cached find: %v", err)
	}
	if got.ID != "cached" {
		t.Fatalf("expected cached snippet, got %s", got.ID)
	}
}

func TestCachedRepository_CacheMiss_NotFound(t *testing.T) {
	ctx := context.Background()
	primary := fake.NewSnippetRepository()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rcli := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	repo := NewSnippetRepository(primary, rcli, time.Minute)

	_, err = repo.FindByID(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestCachedRepository_ExpiredSnippet(t *testing.T) {
	ctx := context.Background()
	primary := fake.NewSnippetRepository()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rcli := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	repo := NewSnippetRepository(primary, rcli, time.Hour)

	now := time.Now().UTC()
	s := domain.Snippet{
		ID:        "exp1",
		Content:   "expires soon",
		CreatedAt: now,
		ExpiresAt: now.Add(2 * time.Second),
	}
	if err := repo.Insert(ctx, s); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Should be cached with short TTL
	got, err := repo.FindByID(ctx, "exp1")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.ID != "exp1" {
		t.Fatalf("wrong id: %s", got.ID)
	}

	// Fast-forward redis time
	mr.FastForward(3 * time.Second)

	// Should be gone from cache now
	_, err = rcli.Get(ctx, keySnippet("exp1")).Result()
	if !errors.Is(err, redis.Nil) {
		t.Fatalf("expected key to expire in cache, got %v", err)
	}
}

func TestCachedRepository_List_Empty(t *testing.T) {
	ctx := context.Background()
	primary := fake.NewSnippetRepository()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rcli := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	repo := NewSnippetRepository(primary, rcli, time.Minute)

	lst, err := repo.List(ctx, 1, 10, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(lst) != 0 {
		t.Fatalf("expected empty list, got %d items", len(lst))
	}

	// Check cache was populated
	k := keyList(1, 10, "")
	val, err := rcli.Get(ctx, k).Result()
	if err != nil {
		t.Fatalf("cache get: %v", err)
	}
	var cached []domain.Snippet
	if err := json.Unmarshal([]byte(val), &cached); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(cached) != 0 {
		t.Fatalf("expected empty cached list")
	}
}

func TestCachedRepository_List_WithTag(t *testing.T) {
	ctx := context.Background()
	primary := fake.NewSnippetRepository()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rcli := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	repo := NewSnippetRepository(primary, rcli, time.Minute)

	now := time.Now().UTC()
	// Insert snippets with different tags
	s1 := domain.Snippet{ID: "go1", Content: "go code", CreatedAt: now, Tags: []string{"go", "code"}}
	s2 := domain.Snippet{ID: "py1", Content: "python code", CreatedAt: now.Add(-time.Hour), Tags: []string{"python", "code"}}
	s3 := domain.Snippet{ID: "go2", Content: "more go", CreatedAt: now.Add(-2 * time.Hour), Tags: []string{"go"}}

	if err := repo.Insert(ctx, s1); err != nil {
		t.Fatalf("insert s1: %v", err)
	}
	if err := repo.Insert(ctx, s2); err != nil {
		t.Fatalf("insert s2: %v", err)
	}
	if err := repo.Insert(ctx, s3); err != nil {
		t.Fatalf("insert s3: %v", err)
	}

	// List with "go" tag
	lst, err := repo.List(ctx, 1, 10, "go")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(lst) != 2 {
		t.Fatalf("expected 2 go snippets, got %d", len(lst))
	}

	// Check cache key is unique per tag
	kGo := keyList(1, 10, "go")
	kPython := keyList(1, 10, "python")
	if kGo == kPython {
		t.Fatalf("cache keys should differ by tag")
	}
}

func TestCachedRepository_List_Pagination(t *testing.T) {
	ctx := context.Background()
	primary := fake.NewSnippetRepository()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rcli := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	repo := NewSnippetRepository(primary, rcli, time.Minute)

	now := time.Now().UTC()
	// Insert 25 snippets
	for i := 0; i < 25; i++ {
		s := domain.Snippet{
			ID:        fmt.Sprintf("s%d", i),
			Content:   fmt.Sprintf("content %d", i),
			CreatedAt: now.Add(time.Duration(-i) * time.Hour),
		}
		if err := repo.Insert(ctx, s); err != nil {
			t.Fatalf("insert s%d: %v", i, err)
		}
	}

	// Get page 1 with limit 10
	page1, err := repo.List(ctx, 1, 10, "")
	if err != nil {
		t.Fatalf("list page 1: %v", err)
	}
	if len(page1) != 10 {
		t.Fatalf("expected 10 items on page 1, got %d", len(page1))
	}

	// Get page 2 with limit 10
	page2, err := repo.List(ctx, 2, 10, "")
	if err != nil {
		t.Fatalf("list page 2: %v", err)
	}
	if len(page2) != 10 {
		t.Fatalf("expected 10 items on page 2, got %d", len(page2))
	}

	// Get page 3 with limit 10 (should have 5 items)
	page3, err := repo.List(ctx, 3, 10, "")
	if err != nil {
		t.Fatalf("list page 3: %v", err)
	}
	if len(page3) != 5 {
		t.Fatalf("expected 5 items on page 3, got %d", len(page3))
	}

	// Ensure different pages are cached separately
	k1 := keyList(1, 10, "")
	k2 := keyList(2, 10, "")
	k3 := keyList(3, 10, "")
	if k1 == k2 || k2 == k3 || k1 == k3 {
		t.Fatalf("cache keys should differ by page")
	}
}

func TestCachedRepository_List_FilterExpired(t *testing.T) {
	ctx := context.Background()
	primary := fake.NewSnippetRepository()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rcli := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	repo := NewSnippetRepository(primary, rcli, time.Minute)

	now := time.Now().UTC()
	// Insert mix of expired and valid snippets
	s1 := domain.Snippet{ID: "valid1", Content: "valid", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	s2 := domain.Snippet{ID: "expired1", Content: "expired", CreatedAt: now.Add(-time.Hour), ExpiresAt: now.Add(-time.Minute)}
	s3 := domain.Snippet{ID: "valid2", Content: "valid", CreatedAt: now.Add(-2 * time.Hour), ExpiresAt: time.Time{}}
	s4 := domain.Snippet{ID: "expired2", Content: "expired", CreatedAt: now.Add(-3 * time.Hour), ExpiresAt: now.Add(-time.Hour)}

	if err := repo.Insert(ctx, s1); err != nil {
		t.Fatalf("insert s1: %v", err)
	}
	if err := repo.Insert(ctx, s2); err != nil {
		t.Fatalf("insert s2: %v", err)
	}
	if err := repo.Insert(ctx, s3); err != nil {
		t.Fatalf("insert s3: %v", err)
	}
	if err := repo.Insert(ctx, s4); err != nil {
		t.Fatalf("insert s4: %v", err)
	}

	// List should filter out expired snippets
	lst, err := repo.List(ctx, 1, 10, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(lst) != 2 {
		t.Fatalf("expected 2 valid snippets, got %d", len(lst))
	}
	for _, s := range lst {
		if s.ID == "expired1" || s.ID == "expired2" {
			t.Fatalf("expired snippet %s should not be in list", s.ID)
		}
	}
}

func TestCachedRepository_List_OrderByCreatedAt(t *testing.T) {
	ctx := context.Background()
	primary := fake.NewSnippetRepository()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rcli := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	repo := NewSnippetRepository(primary, rcli, time.Minute)

	now := time.Now().UTC()
	// Insert snippets in random order
	s1 := domain.Snippet{ID: "old", Content: "old", CreatedAt: now.Add(-3 * time.Hour)}
	s2 := domain.Snippet{ID: "newest", Content: "newest", CreatedAt: now}
	s3 := domain.Snippet{ID: "middle", Content: "middle", CreatedAt: now.Add(-time.Hour)}

	if err := repo.Insert(ctx, s1); err != nil {
		t.Fatalf("insert s1: %v", err)
	}
	if err := repo.Insert(ctx, s2); err != nil {
		t.Fatalf("insert s2: %v", err)
	}
	if err := repo.Insert(ctx, s3); err != nil {
		t.Fatalf("insert s3: %v", err)
	}

	lst, err := repo.List(ctx, 1, 10, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(lst) != 3 {
		t.Fatalf("expected 3 snippets, got %d", len(lst))
	}

	// Check order: newest first
	if lst[0].ID != "newest" {
		t.Fatalf("expected 'newest' first, got %s", lst[0].ID)
	}
	if lst[1].ID != "middle" {
		t.Fatalf("expected 'middle' second, got %s", lst[1].ID)
	}
	if lst[2].ID != "old" {
		t.Fatalf("expected 'old' last, got %s", lst[2].ID)
	}
}

func TestCachedRepository_InvalidateListCache(t *testing.T) {
	ctx := context.Background()
	primary := fake.NewSnippetRepository()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rcli := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	repo := NewSnippetRepository(primary, rcli, time.Minute)

	now := time.Now().UTC()
	// Insert initial snippet
	s1 := domain.Snippet{ID: "s1", Content: "first", CreatedAt: now}
	if err := repo.Insert(ctx, s1); err != nil {
		t.Fatalf("insert s1: %v", err)
	}

	// Populate list cache
	lst1, err := repo.List(ctx, 1, 10, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(lst1) != 1 {
		t.Fatalf("expected 1 item, got %d", len(lst1))
	}

	// Insert new snippet (should invalidate list cache)
	s2 := domain.Snippet{ID: "s2", Content: "second", CreatedAt: now.Add(time.Hour)}
	if err := repo.Insert(ctx, s2); err != nil {
		t.Fatalf("insert s2: %v", err)
	}

	// List should now have 2 items
	lst2, err := repo.List(ctx, 1, 10, "")
	if err != nil {
		t.Fatalf("list after insert: %v", err)
	}
	if len(lst2) != 2 {
		t.Fatalf("expected 2 items after insert, got %d", len(lst2))
	}
}

func TestCachedRepository_RedisError_Fallback(t *testing.T) {
	ctx := context.Background()
	primary := fake.NewSnippetRepository()
	// Use invalid redis address to simulate connection error
	rcli := redis.NewClient(&redis.Options{Addr: "invalid:6379"})
	repo := NewSnippetRepository(primary, rcli, time.Minute)

	now := time.Now().UTC()
	s := domain.Snippet{ID: "fallback", Content: "test", CreatedAt: now}

	// Insert should still work (writes to primary)
	if err := repo.Insert(ctx, s); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// FindByID should fallback to primary
	got, err := repo.FindByID(ctx, "fallback")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.ID != "fallback" {
		t.Fatalf("expected fallback snippet, got %s", got.ID)
	}

	// List should fallback to primary
	lst, err := repo.List(ctx, 1, 10, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(lst) != 1 {
		t.Fatalf("expected 1 item from primary, got %d", len(lst))
	}
}

func TestCachedRepository_KeyHelpers(t *testing.T) {
	// Test snippet key
	k1 := keySnippet("test-id")
	if k1 != "snippet:test-id" {
		t.Fatalf("expected 'snippet:test-id', got %s", k1)
	}

	// Test list key without tag
	k2 := keyList(1, 10, "")
	if k2 != "snippets:p1:l10" {
		t.Fatalf("expected 'snippets:p1:l10', got %s", k2)
	}

	// Test list key with tag
	k3 := keyList(2, 20, "golang")
	if k3 != "snippets:p2:l20:t:golang" {
		t.Fatalf("expected 'snippets:p2:l20:t:golang', got %s", k3)
	}

	// Test different pages have different keys
	k4 := keyList(1, 10, "")
	k5 := keyList(2, 10, "")
	if k4 == k5 {
		t.Fatalf("different pages should have different keys")
	}

	// Test different limits have different keys
	k6 := keyList(1, 10, "")
	k7 := keyList(1, 20, "")
	if k6 == k7 {
		t.Fatalf("different limits should have different keys")
	}
}

func TestCachedRepository_TTLHandling(t *testing.T) {
	ctx := context.Background()
	primary := fake.NewSnippetRepository()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()
	rcli := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	// Test with different cache TTL
	repo1 := NewSnippetRepository(primary, rcli, 10*time.Second)
	repo2 := NewSnippetRepository(primary, rcli, time.Hour)

	now := time.Now().UTC()
	s := domain.Snippet{ID: "ttl-test", Content: "test", CreatedAt: now}

	if err := repo1.Insert(ctx, s); err != nil {
		t.Fatalf("insert repo1: %v", err)
	}

	// Check TTL in redis
	ttl1, err := rcli.TTL(ctx, keySnippet("ttl-test")).Result()
	if err != nil {
		t.Fatalf("get TTL: %v", err)
	}
	if ttl1 > 10*time.Second || ttl1 <= 0 {
		t.Fatalf("expected TTL around 10s, got %v", ttl1)
	}

	// Clear and test with longer TTL
	rcli.FlushAll(ctx)
	if err := repo2.Insert(ctx, s); err != nil {
		t.Fatalf("insert repo2: %v", err)
	}

	ttl2, err := rcli.TTL(ctx, keySnippet("ttl-test")).Result()
	if err != nil {
		t.Fatalf("get TTL: %v", err)
	}
	if ttl2 < 30*time.Minute {
		t.Fatalf("expected TTL around 1h, got %v", ttl2)
	}
}
