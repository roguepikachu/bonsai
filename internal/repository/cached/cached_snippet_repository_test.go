//go:build integration
// +build integration

package cached

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/roguepikachu/bonsai/internal/domain"
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
