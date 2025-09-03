package fake

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/roguepikachu/bonsai/internal/domain"
	"github.com/roguepikachu/bonsai/internal/repository"
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

func TestFakeRepo_Insert_Overwrite(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()
	now := time.Now()

	// Insert initial snippet
	s1 := domain.Snippet{ID: "dup", Content: "first", CreatedAt: now}
	if err := r.Insert(ctx, s1); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// Verify it exists
	got, err := r.FindByID(ctx, "dup")
	if err != nil {
		t.Fatalf("find after first insert: %v", err)
	}
	if got.Content != "first" {
		t.Fatalf("expected content 'first', got %s", got.Content)
	}

	// Overwrite with same ID
	s2 := domain.Snippet{ID: "dup", Content: "second", CreatedAt: now.Add(time.Hour)}
	if err := r.Insert(ctx, s2); err != nil {
		t.Fatalf("second insert: %v", err)
	}

	// Verify it was overwritten
	got, err = r.FindByID(ctx, "dup")
	if err != nil {
		t.Fatalf("find after overwrite: %v", err)
	}
	if got.Content != "second" {
		t.Fatalf("expected content 'second', got %s", got.Content)
	}
}

func TestFakeRepo_FindByID_NotFound(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()

	_, err := r.FindByID(ctx, "nonexistent")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFakeRepo_List_Empty(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()

	got, err := r.List(ctx, 1, 10, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty list, got %d items", len(got))
	}
}

func TestFakeRepo_List_MultiplePages(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()
	now := time.Now()

	// Insert 15 snippets
	for i := 0; i < 15; i++ {
		s := domain.Snippet{
			ID:        fmt.Sprintf("s%02d", i),
			Content:   fmt.Sprintf("content %d", i),
			CreatedAt: now.Add(time.Duration(i) * time.Hour),
		}
		if err := r.Insert(ctx, s); err != nil {
			t.Fatalf("insert s%02d: %v", i, err)
		}
	}

	// Get page 1 with limit 5
	page1, err := r.List(ctx, 1, 5, "")
	if err != nil {
		t.Fatalf("list page 1: %v", err)
	}
	if len(page1) != 5 {
		t.Fatalf("expected 5 items on page 1, got %d", len(page1))
	}

	// Get page 2 with limit 5
	page2, err := r.List(ctx, 2, 5, "")
	if err != nil {
		t.Fatalf("list page 2: %v", err)
	}
	if len(page2) != 5 {
		t.Fatalf("expected 5 items on page 2, got %d", len(page2))
	}

	// Get page 3 with limit 5 (should have 5 items)
	page3, err := r.List(ctx, 3, 5, "")
	if err != nil {
		t.Fatalf("list page 3: %v", err)
	}
	if len(page3) != 5 {
		t.Fatalf("expected 5 items on page 3, got %d", len(page3))
	}

	// Get page 4 with limit 5 (should be empty)
	page4, err := r.List(ctx, 4, 5, "")
	if err != nil {
		t.Fatalf("list page 4: %v", err)
	}
	if len(page4) != 0 {
		t.Fatalf("expected 0 items on page 4, got %d", len(page4))
	}

	// Verify order (newest first)
	if page1[0].ID != "s14" {
		t.Fatalf("expected newest (s14) first, got %s", page1[0].ID)
	}
}

func TestFakeRepo_List_ExpiredFilter(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	r := NewSnippetRepository(WithNow(func() time.Time { return now }))
	ctx := context.Background()

	// Insert mix of expired and valid snippets
	snippets := []domain.Snippet{
		{ID: "valid1", CreatedAt: now, ExpiresAt: future},
		{ID: "expired1", CreatedAt: now.Add(-time.Minute), ExpiresAt: past},
		{ID: "valid2", CreatedAt: now.Add(-2 * time.Minute), ExpiresAt: time.Time{}}, // no expiry
		{ID: "expired2", CreatedAt: now.Add(-3 * time.Minute), ExpiresAt: now},       // expires exactly now
	}

	for _, s := range snippets {
		if err := r.Insert(ctx, s); err != nil {
			t.Fatalf("insert %s: %v", s.ID, err)
		}
	}

	got, err := r.List(ctx, 1, 10, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	// Should only have valid snippets
	if len(got) != 2 {
		t.Fatalf("expected 2 valid snippets, got %d", len(got))
	}

	for _, s := range got {
		if s.ID == "expired1" || s.ID == "expired2" {
			t.Fatalf("expired snippet %s should not be in list", s.ID)
		}
	}
}

func TestFakeRepo_List_MultipleTagFilter(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()
	now := time.Now()

	// Insert snippets with various tag combinations
	snippets := []domain.Snippet{
		{ID: "go1", CreatedAt: now, Tags: []string{"go", "backend"}},
		{ID: "go2", CreatedAt: now.Add(-time.Hour), Tags: []string{"go", "cli"}},
		{ID: "py1", CreatedAt: now.Add(-2 * time.Hour), Tags: []string{"python", "backend"}},
		{ID: "js1", CreatedAt: now.Add(-3 * time.Hour), Tags: []string{"javascript", "frontend"}},
		{ID: "go3", CreatedAt: now.Add(-4 * time.Hour), Tags: []string{"go"}},
	}

	for _, s := range snippets {
		if err := r.Insert(ctx, s); err != nil {
			t.Fatalf("insert %s: %v", s.ID, err)
		}
	}

	// Filter by "go" tag
	goSnippets, err := r.List(ctx, 1, 10, "go")
	if err != nil {
		t.Fatalf("list go: %v", err)
	}
	if len(goSnippets) != 3 {
		t.Fatalf("expected 3 go snippets, got %d", len(goSnippets))
	}

	// Filter by "backend" tag
	backendSnippets, err := r.List(ctx, 1, 10, "backend")
	if err != nil {
		t.Fatalf("list backend: %v", err)
	}
	if len(backendSnippets) != 2 {
		t.Fatalf("expected 2 backend snippets, got %d", len(backendSnippets))
	}

	// Filter by non-existent tag
	noneSnippets, err := r.List(ctx, 1, 10, "rust")
	if err != nil {
		t.Fatalf("list rust: %v", err)
	}
	if len(noneSnippets) != 0 {
		t.Fatalf("expected 0 rust snippets, got %d", len(noneSnippets))
	}
}

func TestFakeRepo_WithOptions(t *testing.T) {
	now := time.Now()
	customTime := now.Add(-24 * time.Hour)

	// Test WithNow option
	r1 := NewSnippetRepository(WithNow(func() time.Time { return customTime }))
	if r1.now() != customTime {
		t.Fatalf("WithNow option not applied")
	}

	// Test WithItems option
	items := []domain.Snippet{
		{ID: "pre1", Content: "preloaded 1", CreatedAt: now},
		{ID: "pre2", Content: "preloaded 2", CreatedAt: now.Add(-time.Hour)},
	}
	r2 := NewSnippetRepository(WithItems(items...))

	ctx := context.Background()
	got, err := r2.FindByID(ctx, "pre1")
	if err != nil {
		t.Fatalf("find pre1: %v", err)
	}
	if got.Content != "preloaded 1" {
		t.Fatalf("expected 'preloaded 1', got %s", got.Content)
	}

	got, err = r2.FindByID(ctx, "pre2")
	if err != nil {
		t.Fatalf("find pre2: %v", err)
	}
	if got.Content != "preloaded 2" {
		t.Fatalf("expected 'preloaded 2', got %s", got.Content)
	}

	// Test multiple options
	r3 := NewSnippetRepository(
		WithNow(func() time.Time { return customTime }),
		WithItems(items...),
	)
	if r3.now() != customTime {
		t.Fatalf("WithNow not applied with multiple options")
	}
	if _, err := r3.FindByID(ctx, "pre1"); err != nil {
		t.Fatalf("WithItems not applied with multiple options: %v", err)
	}
}

func TestFakeRepo_List_OrderByCreatedAtDesc(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()
	now := time.Now()

	// Insert snippets in random order
	snippets := []domain.Snippet{
		{ID: "middle", CreatedAt: now},
		{ID: "newest", CreatedAt: now.Add(2 * time.Hour)},
		{ID: "oldest", CreatedAt: now.Add(-2 * time.Hour)},
		{ID: "second", CreatedAt: now.Add(time.Hour)},
	}

	for _, s := range snippets {
		if err := r.Insert(ctx, s); err != nil {
			t.Fatalf("insert %s: %v", s.ID, err)
		}
	}

	got, err := r.List(ctx, 1, 10, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(got) != 4 {
		t.Fatalf("expected 4 snippets, got %d", len(got))
	}

	// Check order: newest to oldest
	expectedOrder := []string{"newest", "second", "middle", "oldest"}
	for i, id := range expectedOrder {
		if got[i].ID != id {
			t.Fatalf("expected %s at position %d, got %s", id, i, got[i].ID)
		}
	}

	// Verify timestamps are in descending order
	for i := 1; i < len(got); i++ {
		if !got[i-1].CreatedAt.After(got[i].CreatedAt) {
			t.Fatalf("timestamps not in descending order at index %d", i)
		}
	}
}

func TestFakeRepo_List_LimitBoundaries(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()
	now := time.Now()

	// Insert 3 snippets
	for i := 0; i < 3; i++ {
		s := domain.Snippet{ID: fmt.Sprintf("s%d", i), CreatedAt: now.Add(time.Duration(i) * time.Hour)}
		if err := r.Insert(ctx, s); err != nil {
			t.Fatalf("insert s%d: %v", i, err)
		}
	}

	// Test negative limit (should be coerced to 1)
	got, err := r.List(ctx, 1, -5, "")
	if err != nil {
		t.Fatalf("list with negative limit: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 item with negative limit, got %d", len(got))
	}

	// Test zero limit (should be coerced to 1)
	got, err = r.List(ctx, 1, 0, "")
	if err != nil {
		t.Fatalf("list with zero limit: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 item with zero limit, got %d", len(got))
	}

	// Test limit larger than available items
	got, err = r.List(ctx, 1, 100, "")
	if err != nil {
		t.Fatalf("list with large limit: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 items with large limit, got %d", len(got))
	}
}

func TestFakeRepo_List_PageBoundaries(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()
	now := time.Now()

	// Insert 5 snippets
	for i := 0; i < 5; i++ {
		s := domain.Snippet{ID: fmt.Sprintf("s%d", i), CreatedAt: now.Add(time.Duration(i) * time.Hour)}
		if err := r.Insert(ctx, s); err != nil {
			t.Fatalf("insert s%d: %v", i, err)
		}
	}

	// Test negative page (should be coerced to 1)
	got, err := r.List(ctx, -1, 2, "")
	if err != nil {
		t.Fatalf("list with negative page: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 items on negative page, got %d", len(got))
	}

	// Test zero page (should be coerced to 1)
	got, err = r.List(ctx, 0, 2, "")
	if err != nil {
		t.Fatalf("list with zero page: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 items on zero page, got %d", len(got))
	}
}

func TestFakeRepo_DeleteByID(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()
	now := time.Now()

	// Insert snippet
	s := domain.Snippet{ID: "del1", Content: "to delete", CreatedAt: now}
	if err := r.Insert(ctx, s); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Verify it exists
	if _, err := r.FindByID(ctx, "del1"); err != nil {
		t.Fatalf("find before delete: %v", err)
	}

	// Delete it
	r.DeleteByID("del1")

	// Verify it's gone
	_, err := r.FindByID(ctx, "del1")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}

	// Delete non-existent ID (should not panic)
	r.DeleteByID("nonexistent")
}

func TestFakeRepo_ConcurrentAccess(t *testing.T) {
	// Note: This fake is not thread-safe by design, but this test ensures
	// it doesn't panic when used sequentially from multiple goroutines
	r := NewSnippetRepository()
	ctx := context.Background()
	now := time.Now()

	done := make(chan bool, 3)

	// Insert from goroutine
	go func() {
		s := domain.Snippet{ID: "c1", Content: "concurrent 1", CreatedAt: now}
		_ = r.Insert(ctx, s)
		done <- true
	}()

	// Wait for insert
	<-done

	// Find from goroutine
	go func() {
		_, _ = r.FindByID(ctx, "c1")
		done <- true
	}()

	// Wait for find
	<-done

	// List from goroutine
	go func() {
		_, _ = r.List(ctx, 1, 10, "")
		done <- true
	}()

	// Wait for list
	<-done

	// Verify final state
	got, err := r.FindByID(ctx, "c1")
	if err != nil {
		t.Fatalf("final find: %v", err)
	}
	if got.Content != "concurrent 1" {
		t.Fatalf("expected 'concurrent 1', got %s", got.Content)
	}
}

func TestContainsTag(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		want     string
		expected bool
	}{
		{"exact match", []string{"go", "test"}, "go", true},
		{"case insensitive", []string{"Go", "Test"}, "go", true},
		{"mixed case", []string{"GoLang"}, "golang", true},
		{"not found", []string{"python", "java"}, "go", false},
		{"empty tags", []string{}, "go", false},
		{"empty want", []string{"go"}, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsTag(tt.tags, tt.want)
			if got != tt.expected {
				t.Fatalf("containsTag(%v, %q) = %v, want %v", tt.tags, tt.want, got, tt.expected)
			}
		})
	}
}
