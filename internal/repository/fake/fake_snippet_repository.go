// Package fake provides in-memory fakes for repository interfaces for testing.
package fake

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/roguepikachu/bonsai/internal/domain"
	"github.com/roguepikachu/bonsai/internal/repository"
)

// SnippetRepository is an in-memory fake implementing repository.SnippetRepository.
// It's intentionally simple and not concurrency-safe (tests typically run single-threaded).
type SnippetRepository struct {
	byID map[string]domain.Snippet
	now  func() time.Time
}

// Option configures the fake repository.
type Option func(*SnippetRepository)

// WithNow overrides the time source used for expiry filtering.
func WithNow(f func() time.Time) Option { return func(r *SnippetRepository) { r.now = f } }

// WithItems seeds the repository with the provided snippets (by ID).
func WithItems(items ...domain.Snippet) Option {
	return func(r *SnippetRepository) {
		for _, s := range items {
			r.byID[s.ID] = s
		}
	}
}

// NewSnippetRepository creates a new in-memory fake repo.
func NewSnippetRepository(opts ...Option) *SnippetRepository {
	r := &SnippetRepository{byID: make(map[string]domain.Snippet), now: time.Now}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *SnippetRepository) Insert(_ context.Context, s domain.Snippet) error {
	r.byID[s.ID] = s
	return nil
}

func (r *SnippetRepository) FindByID(_ context.Context, id string) (domain.Snippet, error) {
	if s, ok := r.byID[id]; ok {
		return s, nil
	}
	return domain.Snippet{}, repository.ErrNotFound
}

func (r *SnippetRepository) List(_ context.Context, page, limit int, tag string) ([]domain.Snippet, error) {
	now := r.now()
	items := make([]domain.Snippet, 0, len(r.byID))
	for _, s := range r.byID {
		if !s.ExpiresAt.IsZero() && !now.Before(s.ExpiresAt) {
			continue
		}
		if tag != "" && !containsTag(s.Tags, tag) {
			continue
		}
		items = append(items, s)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 1
	}
	start := (page - 1) * limit
	if start >= len(items) {
		return []domain.Snippet{}, nil
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], nil
}

func containsTag(tags []string, want string) bool {
	for _, t := range tags {
		if strings.EqualFold(t, want) {
			return true
		}
	}
	return false
}

var _ repository.SnippetRepository = (*SnippetRepository)(nil)
