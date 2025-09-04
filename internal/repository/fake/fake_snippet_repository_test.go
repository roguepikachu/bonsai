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

func TestFakeRepo_Update_Basic(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()
	now := time.Now()

	// Insert initial snippet
	original := domain.Snippet{
		ID:        "update1",
		Content:   "original content",
		CreatedAt: now,
		Tags:      []string{"original", "tag"},
		ExpiresAt: now.Add(time.Hour),
	}
	if err := r.Insert(ctx, original); err != nil {
		t.Fatalf("insert original: %v", err)
	}

	// Update the snippet
	updated := domain.Snippet{
		ID:        "update1",
		Content:   "updated content",
		CreatedAt: now, // Should preserve this
		Tags:      []string{"updated", "tag"},
		ExpiresAt: now.Add(2 * time.Hour),
	}
	if err := r.Update(ctx, updated); err != nil {
		t.Fatalf("update: %v", err)
	}

	// Verify the update
	got, err := r.FindByID(ctx, "update1")
	if err != nil {
		t.Fatalf("find after update: %v", err)
	}
	if got.Content != "updated content" {
		t.Fatalf("expected 'updated content', got %s", got.Content)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "updated" || got.Tags[1] != "tag" {
		t.Fatalf("expected tags [updated, tag], got %v", got.Tags)
	}
	if !got.ExpiresAt.Equal(now.Add(2 * time.Hour)) {
		t.Fatalf("expected expires at %v, got %v", now.Add(2*time.Hour), got.ExpiresAt)
	}
}

func TestFakeRepo_Update_NotFound(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()
	now := time.Now()

	// Try to update non-existent snippet
	snippet := domain.Snippet{
		ID:        "nonexistent",
		Content:   "some content",
		CreatedAt: now,
	}
	err := r.Update(ctx, snippet)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFakeRepo_Update_EmptyID(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()
	now := time.Now()

	// Try to update with empty ID
	snippet := domain.Snippet{
		ID:        "",
		Content:   "some content",
		CreatedAt: now,
	}
	err := r.Update(ctx, snippet)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for empty ID, got %v", err)
	}
}

func TestFakeRepo_Update_UnicodeContent(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()
	now := time.Now()

	// Insert snippet
	original := domain.Snippet{
		ID:        "unicode1",
		Content:   "simple",
		CreatedAt: now,
	}
	if err := r.Insert(ctx, original); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Update with unicode content
	updated := domain.Snippet{
		ID:        "unicode1",
		Content:   "🚀 こんにちは 世界 🌍 عالم 🎯",
		CreatedAt: now,
	}
	if err := r.Update(ctx, updated); err != nil {
		t.Fatalf("update with unicode: %v", err)
	}

	// Verify update
	got, err := r.FindByID(ctx, "unicode1")
	if err != nil {
		t.Fatalf("find after unicode update: %v", err)
	}
	if got.Content != "🚀 こんにちは 世界 🌍 عالم 🎯" {
		t.Fatalf("unicode content not preserved: %s", got.Content)
	}
}

func TestFakeRepo_Update_LargeContent(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()
	now := time.Now()

	// Insert snippet
	original := domain.Snippet{
		ID:        "large1",
		Content:   "small",
		CreatedAt: now,
	}
	if err := r.Insert(ctx, original); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Create large content (10KB)
	largeContent := make([]byte, 10240)
	for i := range largeContent {
		largeContent[i] = byte('A' + (i % 26))
	}

	// Update with large content
	updated := domain.Snippet{
		ID:        "large1",
		Content:   string(largeContent),
		CreatedAt: now,
	}
	if err := r.Update(ctx, updated); err != nil {
		t.Fatalf("update with large content: %v", err)
	}

	// Verify update
	got, err := r.FindByID(ctx, "large1")
	if err != nil {
		t.Fatalf("find after large update: %v", err)
	}
	if len(got.Content) != 10240 {
		t.Fatalf("expected content length 10240, got %d", len(got.Content))
	}
	if got.Content[:5] != "ABCDE" {
		t.Fatalf("large content pattern incorrect: %s", got.Content[:5])
	}
}

func TestFakeRepo_Update_EmptyContent(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()
	now := time.Now()

	// Insert snippet
	original := domain.Snippet{
		ID:        "empty1",
		Content:   "original content",
		CreatedAt: now,
	}
	if err := r.Insert(ctx, original); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Update with empty content
	updated := domain.Snippet{
		ID:        "empty1",
		Content:   "",
		CreatedAt: now,
	}
	if err := r.Update(ctx, updated); err != nil {
		t.Fatalf("update with empty content: %v", err)
	}

	// Verify update
	got, err := r.FindByID(ctx, "empty1")
	if err != nil {
		t.Fatalf("find after empty update: %v", err)
	}
	if got.Content != "" {
		t.Fatalf("expected empty content, got %s", got.Content)
	}
}

func TestFakeRepo_Update_EmptyTags(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()
	now := time.Now()

	// Insert snippet with tags
	original := domain.Snippet{
		ID:        "tags1",
		Content:   "content",
		CreatedAt: now,
		Tags:      []string{"tag1", "tag2"},
	}
	if err := r.Insert(ctx, original); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Update with empty tags
	updated := domain.Snippet{
		ID:        "tags1",
		Content:   "updated content",
		CreatedAt: now,
		Tags:      []string{},
	}
	if err := r.Update(ctx, updated); err != nil {
		t.Fatalf("update with empty tags: %v", err)
	}

	// Verify update
	got, err := r.FindByID(ctx, "tags1")
	if err != nil {
		t.Fatalf("find after empty tags update: %v", err)
	}
	if len(got.Tags) != 0 {
		t.Fatalf("expected empty tags, got %v", got.Tags)
	}
}

func TestFakeRepo_Update_NilTags(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()
	now := time.Now()

	// Insert snippet with tags
	original := domain.Snippet{
		ID:        "niltags1",
		Content:   "content",
		CreatedAt: now,
		Tags:      []string{"tag1", "tag2"},
	}
	if err := r.Insert(ctx, original); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Update with nil tags
	updated := domain.Snippet{
		ID:        "niltags1",
		Content:   "updated content",
		CreatedAt: now,
		Tags:      nil,
	}
	if err := r.Update(ctx, updated); err != nil {
		t.Fatalf("update with nil tags: %v", err)
	}

	// Verify update
	got, err := r.FindByID(ctx, "niltags1")
	if err != nil {
		t.Fatalf("find after nil tags update: %v", err)
	}
	if len(got.Tags) != 0 {
		t.Fatalf("expected nil or empty tags, got %v", got.Tags)
	}
}

func TestFakeRepo_Update_ManyTags(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()
	now := time.Now()

	// Insert snippet
	original := domain.Snippet{
		ID:        "manytags1",
		Content:   "content",
		CreatedAt: now,
		Tags:      []string{"tag1"},
	}
	if err := r.Insert(ctx, original); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Update with many tags
	manyTags := make([]string, 50)
	for i := range manyTags {
		manyTags[i] = fmt.Sprintf("tag%d", i)
	}

	updated := domain.Snippet{
		ID:        "manytags1",
		Content:   "updated content",
		CreatedAt: now,
		Tags:      manyTags,
	}
	if err := r.Update(ctx, updated); err != nil {
		t.Fatalf("update with many tags: %v", err)
	}

	// Verify update
	got, err := r.FindByID(ctx, "manytags1")
	if err != nil {
		t.Fatalf("find after many tags update: %v", err)
	}
	if len(got.Tags) != 50 {
		t.Fatalf("expected 50 tags, got %d", len(got.Tags))
	}
	if got.Tags[0] != "tag0" || got.Tags[49] != "tag49" {
		t.Fatalf("tag ordering incorrect: first=%s, last=%s", got.Tags[0], got.Tags[len(got.Tags)-1])
	}
}

func TestFakeRepo_Update_SpecialCharacterTags(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()
	now := time.Now()

	// Insert snippet
	original := domain.Snippet{
		ID:        "special1",
		Content:   "content",
		CreatedAt: now,
		Tags:      []string{"normal"},
	}
	if err := r.Insert(ctx, original); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Update with special character tags
	updated := domain.Snippet{
		ID:        "special1",
		Content:   "updated content",
		CreatedAt: now,
		Tags:      []string{"tag-with-dashes", "tag_with_underscores", "tag.with.dots", "tag@with@symbols", "🚀emoji-tag"},
	}
	if err := r.Update(ctx, updated); err != nil {
		t.Fatalf("update with special tags: %v", err)
	}

	// Verify update
	got, err := r.FindByID(ctx, "special1")
	if err != nil {
		t.Fatalf("find after special tags update: %v", err)
	}
	expectedTags := []string{"tag-with-dashes", "tag_with_underscores", "tag.with.dots", "tag@with@symbols", "🚀emoji-tag"}
	if len(got.Tags) != len(expectedTags) {
		t.Fatalf("expected %d tags, got %d", len(expectedTags), len(got.Tags))
	}
	for i, expected := range expectedTags {
		if got.Tags[i] != expected {
			t.Fatalf("expected tag %s at position %d, got %s", expected, i, got.Tags[i])
		}
	}
}

func TestFakeRepo_Update_ZeroExpiresAt(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()
	now := time.Now()

	// Insert snippet with expiration
	original := domain.Snippet{
		ID:        "zeroexp1",
		Content:   "content",
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	}
	if err := r.Insert(ctx, original); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Update with zero ExpiresAt (no expiration)
	updated := domain.Snippet{
		ID:        "zeroexp1",
		Content:   "updated content",
		CreatedAt: now,
		ExpiresAt: time.Time{}, // Zero value means no expiration
	}
	if err := r.Update(ctx, updated); err != nil {
		t.Fatalf("update with zero expires: %v", err)
	}

	// Verify update
	got, err := r.FindByID(ctx, "zeroexp1")
	if err != nil {
		t.Fatalf("find after zero expires update: %v", err)
	}
	if !got.ExpiresAt.IsZero() {
		t.Fatalf("expected zero ExpiresAt, got %v", got.ExpiresAt)
	}
}

func TestFakeRepo_Update_FutureExpiresAt(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()
	now := time.Now()

	// Insert snippet
	original := domain.Snippet{
		ID:        "future1",
		Content:   "content",
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	}
	if err := r.Insert(ctx, original); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Update with far future expiration
	futureTime := now.Add(365 * 24 * time.Hour) // 1 year
	updated := domain.Snippet{
		ID:        "future1",
		Content:   "updated content",
		CreatedAt: now,
		ExpiresAt: futureTime,
	}
	if err := r.Update(ctx, updated); err != nil {
		t.Fatalf("update with future expires: %v", err)
	}

	// Verify update
	got, err := r.FindByID(ctx, "future1")
	if err != nil {
		t.Fatalf("find after future expires update: %v", err)
	}
	if !got.ExpiresAt.Equal(futureTime) {
		t.Fatalf("expected ExpiresAt %v, got %v", futureTime, got.ExpiresAt)
	}
}

func TestFakeRepo_Update_PastExpiresAt(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()
	now := time.Now()

	// Insert snippet
	original := domain.Snippet{
		ID:        "past1",
		Content:   "content",
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	}
	if err := r.Insert(ctx, original); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Update with past expiration (already expired)
	pastTime := now.Add(-time.Hour)
	updated := domain.Snippet{
		ID:        "past1",
		Content:   "updated content",
		CreatedAt: now,
		ExpiresAt: pastTime,
	}
	if err := r.Update(ctx, updated); err != nil {
		t.Fatalf("update with past expires: %v", err)
	}

	// Verify update (should still work even if expired)
	got, err := r.FindByID(ctx, "past1")
	if err != nil {
		t.Fatalf("find after past expires update: %v", err)
	}
	if !got.ExpiresAt.Equal(pastTime) {
		t.Fatalf("expected ExpiresAt %v, got %v", pastTime, got.ExpiresAt)
	}
}

func TestFakeRepo_Update_PreservesCreatedAt(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()
	originalTime := time.Now().Add(-2 * time.Hour)
	updateTime := time.Now()

	// Insert snippet
	original := domain.Snippet{
		ID:        "preserve1",
		Content:   "content",
		CreatedAt: originalTime,
	}
	if err := r.Insert(ctx, original); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Update with different CreatedAt (should preserve original)
	updated := domain.Snippet{
		ID:        "preserve1",
		Content:   "updated content",
		CreatedAt: updateTime, // This should be ignored
	}
	if err := r.Update(ctx, updated); err != nil {
		t.Fatalf("update: %v", err)
	}

	// Verify CreatedAt was preserved
	got, err := r.FindByID(ctx, "preserve1")
	if err != nil {
		t.Fatalf("find after update: %v", err)
	}
	if !got.CreatedAt.Equal(originalTime) {
		t.Fatalf("expected CreatedAt %v, got %v", originalTime, got.CreatedAt)
	}
	if got.Content != "updated content" {
		t.Fatalf("expected updated content, got %s", got.Content)
	}
}

func TestFakeRepo_Update_MultipleUpdates(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()
	now := time.Now()

	// Insert snippet
	original := domain.Snippet{
		ID:        "multiple1",
		Content:   "version1",
		CreatedAt: now,
		Tags:      []string{"v1"},
	}
	if err := r.Insert(ctx, original); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// First update
	update1 := domain.Snippet{
		ID:        "multiple1",
		Content:   "version2",
		CreatedAt: now,
		Tags:      []string{"v1", "v2"},
	}
	if err := r.Update(ctx, update1); err != nil {
		t.Fatalf("first update: %v", err)
	}

	// Second update
	update2 := domain.Snippet{
		ID:        "multiple1",
		Content:   "version3",
		CreatedAt: now,
		Tags:      []string{"v3"},
		ExpiresAt: now.Add(time.Hour),
	}
	if err := r.Update(ctx, update2); err != nil {
		t.Fatalf("second update: %v", err)
	}

	// Third update
	update3 := domain.Snippet{
		ID:        "multiple1",
		Content:   "final version",
		CreatedAt: now,
		Tags:      []string{"final"},
		ExpiresAt: time.Time{}, // Remove expiration
	}
	if err := r.Update(ctx, update3); err != nil {
		t.Fatalf("third update: %v", err)
	}

	// Verify final state
	got, err := r.FindByID(ctx, "multiple1")
	if err != nil {
		t.Fatalf("find after multiple updates: %v", err)
	}
	if got.Content != "final version" {
		t.Fatalf("expected 'final version', got %s", got.Content)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "final" {
		t.Fatalf("expected tags [final], got %v", got.Tags)
	}
	if !got.ExpiresAt.IsZero() {
		t.Fatalf("expected no expiration, got %v", got.ExpiresAt)
	}
	if !got.CreatedAt.Equal(now) {
		t.Fatalf("CreatedAt should be preserved: expected %v, got %v", now, got.CreatedAt)
	}
}

func TestFakeRepo_Update_WhitespaceContent(t *testing.T) {
	r := NewSnippetRepository()
	ctx := context.Background()
	now := time.Now()

	// Insert snippet
	original := domain.Snippet{
		ID:        "whitespace1",
		Content:   "normal content",
		CreatedAt: now,
	}
	if err := r.Insert(ctx, original); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Update with various whitespace content
	updated := domain.Snippet{
		ID:        "whitespace1",
		Content:   "\t\n  \r\n\t  ",
		CreatedAt: now,
	}
	if err := r.Update(ctx, updated); err != nil {
		t.Fatalf("update with whitespace: %v", err)
	}

	// Verify whitespace is preserved
	got, err := r.FindByID(ctx, "whitespace1")
	if err != nil {
		t.Fatalf("find after whitespace update: %v", err)
	}
	if got.Content != "\t\n  \r\n\t  " {
		t.Fatalf("whitespace not preserved: %q", got.Content)
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
