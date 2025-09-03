package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/roguepikachu/bonsai/internal/domain"
	"github.com/roguepikachu/bonsai/internal/repository"
)

type stubClock struct{ t time.Time }

func (s stubClock) Now() time.Time { return s.t }

type fakeRepo struct {
	mu           sync.RWMutex
	inserted     []domain.Snippet
	findByID     map[string]domain.Snippet
	listSnippets []domain.Snippet
	listArgs     struct {
		page, limit int
		tag         string
	}
	insertErr  error
	findErr    error
	listErr    error
	insertCall int
	findCall   int
	listCall   int
}

func (f *fakeRepo) Insert(_ context.Context, s domain.Snippet) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.insertCall++
	if f.insertErr != nil {
		return f.insertErr
	}
	f.inserted = append(f.inserted, s)
	if f.findByID == nil {
		f.findByID = map[string]domain.Snippet{}
	}
	f.findByID[s.ID] = s
	return nil
}

func (f *fakeRepo) FindByID(_ context.Context, id string) (domain.Snippet, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	f.findCall++
	if f.findErr != nil {
		return domain.Snippet{}, f.findErr
	}
	if s, ok := f.findByID[id]; ok {
		return s, nil
	}
	return domain.Snippet{}, repository.ErrNotFound
}

func (f *fakeRepo) List(_ context.Context, page, limit int, tag string) ([]domain.Snippet, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	f.listCall++
	f.listArgs.page, f.listArgs.limit, f.listArgs.tag = page, limit, tag
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listSnippets, nil
}

func TestCreateSnippet_NoExpiry(t *testing.T) {
	fixed := time.Date(2025, 8, 30, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepo{}
	s := NewServiceWithOptions(repo, stubClock{t: fixed}, WithIDGenerator(func() string { return "id-123" }))

	got, err := s.CreateSnippet(context.Background(), "hello", 0, []string{"a"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.ID != "id-123" {
		t.Fatalf("want id id-123, got %s", got.ID)
	}
	if !got.CreatedAt.Equal(fixed) {
		t.Fatalf("createdAt mismatch")
	}
	if !got.ExpiresAt.IsZero() {
		t.Fatalf("expected no expiry")
	}
	if len(repo.inserted) != 1 {
		t.Fatalf("expected insert called")
	}
}

func TestGetSnippetByID_NotFound(t *testing.T) {
	repo := &fakeRepo{findByID: map[string]domain.Snippet{}}
	s := NewServiceWithOptions(repo, stubClock{t: time.Now()})
	_, _, err := s.GetSnippetByID(context.Background(), "nope")
	if !errors.Is(err, ErrSnippetNotFound) {
		t.Fatalf("expected ErrSnippetNotFound, got %v", err)
	}
}

func TestListSnippets_Caps(t *testing.T) {
	repo := &fakeRepo{}
	s := NewServiceWithOptions(repo, stubClock{t: time.Now()})
	_, _ = s.ListSnippets(context.Background(), 0, 10000, "tag")
	if repo.listArgs.page != ServiceDefaultPage {
		t.Fatalf("want page=%d got %d", ServiceDefaultPage, repo.listArgs.page)
	}
	if repo.listArgs.limit != ServiceMaxLimit {
		t.Fatalf("want limit=%d got %d", ServiceMaxLimit, repo.listArgs.limit)
	}
	if repo.listArgs.tag != "tag" {
		t.Fatalf("want tag=tag got %s", repo.listArgs.tag)
	}
}

func TestCreateSnippet_WithExpiry(t *testing.T) {
	fixed := time.Date(2025, 8, 31, 10, 0, 0, 0, time.UTC)
	repo := &fakeRepo{}
	s := NewServiceWithOptions(repo, stubClock{t: fixed}, WithIDGenerator(func() string { return "id-exp" }))

	got, err := s.CreateSnippet(context.Background(), "hello", 120, []string{"t"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.ExpiresAt.IsZero() || !got.ExpiresAt.Equal(fixed.Add(120*time.Second)) {
		t.Fatalf("expiresAt mismatch: %v", got.ExpiresAt)
	}
	if len(repo.inserted) != 1 || repo.inserted[0].ID != "id-exp" {
		t.Fatalf("insert not recorded correctly: %+v", repo.inserted)
	}
}

func TestGetSnippetByID_Expired(t *testing.T) {
	now := time.Date(2025, 8, 31, 11, 0, 0, 0, time.UTC)
	past := now.Add(-time.Minute)
	repo := &fakeRepo{findByID: map[string]domain.Snippet{
		"x": {ID: "x", CreatedAt: past.Add(-time.Hour), ExpiresAt: past},
	}}
	s := NewServiceWithOptions(repo, stubClock{t: now})
	_, _, err := s.GetSnippetByID(context.Background(), "x")
	if !errors.Is(err, ErrSnippetExpired) {
		t.Fatalf("expected ErrSnippetExpired, got %v", err)
	}
}

func TestListSnippets_PassesParams(t *testing.T) {
	repo := &fakeRepo{}
	s := NewServiceWithOptions(repo, stubClock{t: time.Now()})
	_, _ = s.ListSnippets(context.Background(), 2, 5, "go")
	if repo.listArgs.page != 2 || repo.listArgs.limit != 5 || repo.listArgs.tag != "go" {
		t.Fatalf("args mismatch: %+v", repo.listArgs)
	}
}

func TestCreateSnippet_EmptyContent(t *testing.T) {
	fixed := time.Date(2025, 8, 30, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepo{}
	s := NewServiceWithOptions(repo, stubClock{t: fixed}, WithIDGenerator(func() string { return "empty-id" }))

	got, err := s.CreateSnippet(context.Background(), "", 0, []string{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Content != "" {
		t.Fatalf("expected empty content, got %q", got.Content)
	}
	if len(got.Tags) != 0 {
		t.Fatalf("expected no tags, got %v", got.Tags)
	}
	if repo.insertCall != 1 {
		t.Fatalf("expected insert called once, got %d", repo.insertCall)
	}
}

func TestCreateSnippet_LargeContent(t *testing.T) {
	fixed := time.Date(2025, 8, 30, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepo{}
	s := NewServiceWithOptions(repo, stubClock{t: fixed}, WithIDGenerator(func() string { return "large-id" }))

	largeContent := ""
	for i := 0; i < 10000; i++ {
		largeContent += "a"
	}

	got, err := s.CreateSnippet(context.Background(), largeContent, 0, []string{"large"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got.Content) != 10000 {
		t.Fatalf("expected content length 10000, got %d", len(got.Content))
	}
}

func TestCreateSnippet_MultipleTags(t *testing.T) {
	fixed := time.Date(2025, 8, 30, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepo{}
	s := NewServiceWithOptions(repo, stubClock{t: fixed}, WithIDGenerator(func() string { return "tags-id" }))

	tags := []string{"go", "testing", "unit", "service", "snippet"}
	got, err := s.CreateSnippet(context.Background(), "test content", 0, tags)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got.Tags) != 5 {
		t.Fatalf("expected 5 tags, got %d", len(got.Tags))
	}
	for i, tag := range tags {
		if got.Tags[i] != tag {
			t.Fatalf("expected tag %s at index %d, got %s", tag, i, got.Tags[i])
		}
	}
}

func TestCreateSnippet_RepositoryError(t *testing.T) {
	fixed := time.Date(2025, 8, 30, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepo{insertErr: fmt.Errorf("database connection lost")}
	s := NewServiceWithOptions(repo, stubClock{t: fixed}, WithIDGenerator(func() string { return "err-id" }))

	_, err := s.CreateSnippet(context.Background(), "content", 60, []string{"error"})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "database connection lost" {
		t.Fatalf("expected database error, got %v", err)
	}
	if len(repo.inserted) != 0 {
		t.Fatalf("expected no inserts on error, got %d", len(repo.inserted))
	}
}

func TestCreateSnippet_NegativeExpiry(t *testing.T) {
	fixed := time.Date(2025, 8, 30, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepo{}
	s := NewServiceWithOptions(repo, stubClock{t: fixed}, WithIDGenerator(func() string { return "neg-exp-id" }))

	got, err := s.CreateSnippet(context.Background(), "content", -100, []string{"negative"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !got.ExpiresAt.IsZero() {
		t.Fatalf("expected no expiry for negative value, got %v", got.ExpiresAt)
	}
}

func TestCreateSnippet_VeryLargeExpiry(t *testing.T) {
	fixed := time.Date(2025, 8, 30, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepo{}
	s := NewServiceWithOptions(repo, stubClock{t: fixed}, WithIDGenerator(func() string { return "large-exp-id" }))

	// 10 years in seconds
	largeExpiry := 10 * 365 * 24 * 60 * 60
	got, err := s.CreateSnippet(context.Background(), "content", largeExpiry, []string{"long"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	expectedExpiry := fixed.Add(time.Duration(largeExpiry) * time.Second)
	if !got.ExpiresAt.Equal(expectedExpiry) {
		t.Fatalf("expected expiry at %v, got %v", expectedExpiry, got.ExpiresAt)
	}
}

func TestCreateSnippet_NilIDGenerator(t *testing.T) {
	fixed := time.Date(2025, 8, 30, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepo{}
	// Explicitly not setting ID generator to test default behavior
	s := &Service{repo: repo, clock: stubClock{t: fixed}, idGen: nil}

	got, err := s.CreateSnippet(context.Background(), "test", 0, []string{"default"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.ID == "" {
		t.Fatalf("expected non-empty ID with default generator")
	}
	// Default uses UUID, should have the standard format
	if len(got.ID) != 36 {
		t.Fatalf("expected UUID format (36 chars), got %d chars: %s", len(got.ID), got.ID)
	}
}

func TestGetSnippetByID_Found(t *testing.T) {
	now := time.Date(2025, 8, 31, 11, 0, 0, 0, time.UTC)
	snippet := domain.Snippet{
		ID:        "found-id",
		Content:   "found content",
		Tags:      []string{"test"},
		CreatedAt: now.Add(-time.Hour),
		ExpiresAt: now.Add(time.Hour),
	}
	repo := &fakeRepo{findByID: map[string]domain.Snippet{
		"found-id": snippet,
	}}
	s := NewServiceWithOptions(repo, stubClock{t: now})

	got, meta, err := s.GetSnippetByID(context.Background(), "found-id")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.ID != "found-id" {
		t.Fatalf("expected ID found-id, got %s", got.ID)
	}
	if meta.CacheStatus != CacheMiss {
		t.Fatalf("expected cache miss, got %s", meta.CacheStatus)
	}
	if repo.findCall != 1 {
		t.Fatalf("expected FindByID called once, got %d", repo.findCall)
	}
}

func TestGetSnippetByID_NoExpiry(t *testing.T) {
	now := time.Date(2025, 8, 31, 11, 0, 0, 0, time.UTC)
	snippet := domain.Snippet{
		ID:        "no-exp",
		Content:   "content",
		Tags:      []string{"permanent"},
		CreatedAt: now.Add(-time.Hour * 24 * 365), // 1 year old
		ExpiresAt: time.Time{},                    // no expiry
	}
	repo := &fakeRepo{findByID: map[string]domain.Snippet{
		"no-exp": snippet,
	}}
	s := NewServiceWithOptions(repo, stubClock{t: now})

	got, _, err := s.GetSnippetByID(context.Background(), "no-exp")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.ID != "no-exp" {
		t.Fatalf("expected ID no-exp, got %s", got.ID)
	}
}

func TestGetSnippetByID_RepositoryError(t *testing.T) {
	now := time.Date(2025, 8, 31, 11, 0, 0, 0, time.UTC)
	repo := &fakeRepo{findErr: fmt.Errorf("connection timeout")}
	s := NewServiceWithOptions(repo, stubClock{t: now})

	_, _, err := s.GetSnippetByID(context.Background(), "any-id")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "find by id: connection timeout" {
		t.Fatalf("expected wrapped error, got %v", err)
	}
}

func TestGetSnippetByID_ExactlyAtExpiry(t *testing.T) {
	now := time.Date(2025, 8, 31, 11, 0, 0, 0, time.UTC)
	snippet := domain.Snippet{
		ID:        "exact-exp",
		Content:   "content",
		CreatedAt: now.Add(-time.Hour),
		ExpiresAt: now, // expires exactly now
	}
	repo := &fakeRepo{findByID: map[string]domain.Snippet{
		"exact-exp": snippet,
	}}
	s := NewServiceWithOptions(repo, stubClock{t: now})

	// Should not be expired when time is exactly at expiry
	got, _, err := s.GetSnippetByID(context.Background(), "exact-exp")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.ID != "exact-exp" {
		t.Fatalf("expected ID exact-exp, got %s", got.ID)
	}
}

func TestGetSnippetByID_JustAfterExpiry(t *testing.T) {
	now := time.Date(2025, 8, 31, 11, 0, 1, 0, time.UTC) // 1 second after
	snippet := domain.Snippet{
		ID:        "just-exp",
		Content:   "content",
		CreatedAt: now.Add(-time.Hour),
		ExpiresAt: now.Add(-time.Second), // expired 1 second ago
	}
	repo := &fakeRepo{findByID: map[string]domain.Snippet{
		"just-exp": snippet,
	}}
	s := NewServiceWithOptions(repo, stubClock{t: now})

	_, _, err := s.GetSnippetByID(context.Background(), "just-exp")
	if !errors.Is(err, ErrSnippetExpired) {
		t.Fatalf("expected ErrSnippetExpired, got %v", err)
	}
}

func TestListSnippets_EmptyList(t *testing.T) {
	repo := &fakeRepo{listSnippets: []domain.Snippet{}}
	s := NewServiceWithOptions(repo, stubClock{t: time.Now()})

	got, err := s.ListSnippets(context.Background(), 1, 10, "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty list, got %d items", len(got))
	}
	if repo.listCall != 1 {
		t.Fatalf("expected List called once, got %d", repo.listCall)
	}
}

func TestListSnippets_WithResults(t *testing.T) {
	now := time.Now()
	snippets := []domain.Snippet{
		{ID: "1", Content: "first", CreatedAt: now},
		{ID: "2", Content: "second", CreatedAt: now.Add(-time.Hour)},
		{ID: "3", Content: "third", CreatedAt: now.Add(-time.Hour * 2)},
	}
	repo := &fakeRepo{listSnippets: snippets}
	s := NewServiceWithOptions(repo, stubClock{t: now})

	got, err := s.ListSnippets(context.Background(), 1, 10, "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 results, got %d", len(got))
	}
	for i, snippet := range snippets {
		if got[i].ID != snippet.ID {
			t.Fatalf("expected ID %s at index %d, got %s", snippet.ID, i, got[i].ID)
		}
	}
}

func TestListSnippets_ZeroPage(t *testing.T) {
	repo := &fakeRepo{}
	s := NewServiceWithOptions(repo, stubClock{t: time.Now()})

	_, _ = s.ListSnippets(context.Background(), 0, 20, "")
	if repo.listArgs.page != ServiceDefaultPage {
		t.Fatalf("expected page normalized to %d, got %d", ServiceDefaultPage, repo.listArgs.page)
	}
}

func TestListSnippets_NegativePage(t *testing.T) {
	repo := &fakeRepo{}
	s := NewServiceWithOptions(repo, stubClock{t: time.Now()})

	_, _ = s.ListSnippets(context.Background(), -5, 20, "")
	if repo.listArgs.page != ServiceDefaultPage {
		t.Fatalf("expected page normalized to %d, got %d", ServiceDefaultPage, repo.listArgs.page)
	}
}

func TestListSnippets_ZeroLimit(t *testing.T) {
	repo := &fakeRepo{}
	s := NewServiceWithOptions(repo, stubClock{t: time.Now()})

	_, _ = s.ListSnippets(context.Background(), 1, 0, "")
	if repo.listArgs.limit != ServiceDefaultLimit {
		t.Fatalf("expected limit normalized to %d, got %d", ServiceDefaultLimit, repo.listArgs.limit)
	}
}

func TestListSnippets_NegativeLimit(t *testing.T) {
	repo := &fakeRepo{}
	s := NewServiceWithOptions(repo, stubClock{t: time.Now()})

	_, _ = s.ListSnippets(context.Background(), 1, -10, "")
	if repo.listArgs.limit != ServiceDefaultLimit {
		t.Fatalf("expected limit normalized to %d, got %d", ServiceDefaultLimit, repo.listArgs.limit)
	}
}

func TestListSnippets_ExceedsMaxLimit(t *testing.T) {
	repo := &fakeRepo{}
	s := NewServiceWithOptions(repo, stubClock{t: time.Now()})

	_, _ = s.ListSnippets(context.Background(), 1, 1000, "")
	if repo.listArgs.limit != ServiceMaxLimit {
		t.Fatalf("expected limit capped at %d, got %d", ServiceMaxLimit, repo.listArgs.limit)
	}
}

func TestListSnippets_RepositoryError(t *testing.T) {
	repo := &fakeRepo{listErr: fmt.Errorf("query failed")}
	s := NewServiceWithOptions(repo, stubClock{t: time.Now()})

	_, err := s.ListSnippets(context.Background(), 1, 10, "test")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "query failed" {
		t.Fatalf("expected query failed error, got %v", err)
	}
}

func TestListSnippets_WithTagFilter(t *testing.T) {
	repo := &fakeRepo{}
	s := NewServiceWithOptions(repo, stubClock{t: time.Now()})

	_, _ = s.ListSnippets(context.Background(), 2, 50, "golang")
	if repo.listArgs.tag != "golang" {
		t.Fatalf("expected tag filter 'golang', got %q", repo.listArgs.tag)
	}
	if repo.listArgs.page != 2 {
		t.Fatalf("expected page 2, got %d", repo.listArgs.page)
	}
	if repo.listArgs.limit != 50 {
		t.Fatalf("expected limit 50, got %d", repo.listArgs.limit)
	}
}

func TestListSnippets_EmptyTag(t *testing.T) {
	repo := &fakeRepo{}
	s := NewServiceWithOptions(repo, stubClock{t: time.Now()})

	_, _ = s.ListSnippets(context.Background(), 1, 10, "")
	if repo.listArgs.tag != "" {
		t.Fatalf("expected empty tag, got %q", repo.listArgs.tag)
	}
}

func TestService_ConcurrentAccess(t *testing.T) {
	repo := &fakeRepo{}
	s := NewServiceWithOptions(repo, stubClock{t: time.Now()}, WithIDGenerator(func() string {
		return fmt.Sprintf("id-%d", time.Now().UnixNano())
	}))

	ctx := context.Background()
	done := make(chan bool, 3)

	// Concurrent create
	go func() {
		_, _ = s.CreateSnippet(ctx, "content1", 60, []string{"concurrent"})
		done <- true
	}()

	// Concurrent list
	go func() {
		_, _ = s.ListSnippets(ctx, 1, 10, "test")
		done <- true
	}()

	// Concurrent get
	go func() {
		_, _, _ = s.GetSnippetByID(ctx, "some-id")
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	// Verify all operations were called
	if repo.insertCall < 1 {
		t.Fatalf("expected at least 1 insert call, got %d", repo.insertCall)
	}
	if repo.listCall < 1 {
		t.Fatalf("expected at least 1 list call, got %d", repo.listCall)
	}
	if repo.findCall < 1 {
		t.Fatalf("expected at least 1 find call, got %d", repo.findCall)
	}
}

func TestCreateSnippet_ContextCancellation(t *testing.T) {
	fixed := time.Date(2025, 8, 30, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepo{}
	s := NewServiceWithOptions(repo, stubClock{t: fixed}, WithIDGenerator(func() string { return "ctx-id" }))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Should still work as our fake repo doesn't check context
	_, err := s.CreateSnippet(ctx, "content", 0, []string{"cancelled"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestNewService(t *testing.T) {
	repo := &fakeRepo{}
	clock := stubClock{t: time.Now()}
	s := NewService(repo, clock)

	if s.repo != repo {
		t.Fatalf("expected repo to be set")
	}
	if s.clock != clock {
		t.Fatalf("expected clock to be set")
	}
	if s.idGen == nil {
		t.Fatalf("expected default ID generator to be set")
	}
}

func TestSnippetMeta_Values(t *testing.T) {
	// Test cache status constants
	if CacheMiss != "MISS" {
		t.Fatalf("expected CacheMiss to be 'MISS', got %s", CacheMiss)
	}
	if CacheHit != "HIT" {
		t.Fatalf("expected CacheHit to be 'HIT', got %s", CacheHit)
	}

	// Test meta struct
	meta := SnippetMeta{CacheStatus: CacheHit}
	if meta.CacheStatus != "HIT" {
		t.Fatalf("expected cache status HIT, got %s", meta.CacheStatus)
	}
}
