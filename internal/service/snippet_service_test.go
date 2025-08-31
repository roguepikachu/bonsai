package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/roguepikachu/bonsai/internal/domain"
	"github.com/roguepikachu/bonsai/internal/repository"
)

type stubClock struct{ t time.Time }

func (s stubClock) Now() time.Time { return s.t }

type fakeRepo struct {
	inserted []domain.Snippet
	findByID map[string]domain.Snippet
	listArgs struct {
		page, limit int
		tag         string
	}
}

func (f *fakeRepo) Insert(_ context.Context, s domain.Snippet) error {
	f.inserted = append(f.inserted, s)
	if f.findByID == nil {
		f.findByID = map[string]domain.Snippet{}
	}
	f.findByID[s.ID] = s
	return nil
}

func (f *fakeRepo) FindByID(_ context.Context, id string) (domain.Snippet, error) {
	if s, ok := f.findByID[id]; ok {
		return s, nil
	}
	return domain.Snippet{}, repository.ErrNotFound
}

func (f *fakeRepo) List(_ context.Context, page, limit int, tag string) ([]domain.Snippet, error) {
	f.listArgs.page, f.listArgs.limit, f.listArgs.tag = page, limit, tag
	return nil, nil
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
