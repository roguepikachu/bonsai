package fake

import (
	"context"
	"testing"
	"time"

	"github.com/roguepikachu/bonsai/internal/domain"
)

func TestFakeRepo_List_FilterAndExpiry(t *testing.T) {
	r := NewSnippetRepository()
	now := time.Now()
	_ = r.Insert(context.Background(), domain.Snippet{ID: "1", CreatedAt: now, Tags: []string{"go"}})
	_ = r.Insert(context.Background(), domain.Snippet{ID: "2", CreatedAt: now.Add(time.Second), Tags: []string{"go", "web"}})
	_ = r.Insert(context.Background(), domain.Snippet{ID: "3", CreatedAt: now, ExpiresAt: now.Add(-time.Minute)})

	got, err := r.List(context.Background(), 1, 10, "go")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 items, got %d", len(got))
	}
	if got[0].ID != "2" {
		t.Fatalf("want newest first")
	}
}

func TestFakeRepo_List_PaginationBounds(t *testing.T) {
	r := NewSnippetRepository()
	now := time.Now()
	for i := 0; i < 5; i++ {
		_ = r.Insert(context.Background(), domain.Snippet{ID: string(rune('a' + i)), CreatedAt: now.Add(time.Duration(i) * time.Second)})
	}
	// page beyond range should return empty
	got, err := r.List(context.Background(), 10, 2, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want 0, got %d", len(got))
	}

	// limit < 1 coerced to 1
	got, err = r.List(context.Background(), 1, 0, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1, got %d", len(got))
	}
}

func TestFakeRepo_List_TagCaseInsensitive(t *testing.T) {
	r := NewSnippetRepository()
	now := time.Now()
	_ = r.Insert(context.Background(), domain.Snippet{ID: "x", CreatedAt: now, Tags: []string{"Go"}})
	got, err := r.List(context.Background(), 1, 10, "go")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].ID != "x" {
		t.Fatalf("tag filter failed: %+v", got)
	}
}
