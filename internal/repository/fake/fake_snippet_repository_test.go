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
