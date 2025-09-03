// Package service contains business logic for the application.
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/roguepikachu/bonsai/internal/domain"
	"github.com/roguepikachu/bonsai/internal/repository"
)

// NewService creates a new Service with the given SnippetRepository and Clock.
func NewService(repo repository.SnippetRepository, clock Clock) *Service {
	return NewServiceWithOptions(repo, clock)
}

// Service provides snippet-related business logic.
type Service struct {
	repo  repository.SnippetRepository
	clock Clock
	idGen func() string
}

// Error variables
var (
	ErrSnippetNotFound = errors.New("snippet not found")
	ErrSnippetExpired  = errors.New("snippet expired")
)

// Option configures Service.
type Option func(*Service)

// WithIDGenerator overrides the snippet ID generator.
func WithIDGenerator(f func() string) Option { return func(s *Service) { s.idGen = f } }

// NewServiceWithOptions creates a Service with additional options for testability.
func NewServiceWithOptions(repo repository.SnippetRepository, clock Clock, opts ...Option) *Service {
	s := &Service{repo: repo, clock: clock, idGen: generateID}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// generateID returns a new unique ID for a snippet.
func generateID() string {
	return uuid.New().String()
}

// CreateSnippet creates a new snippet with content, expiry, and tags.
func (s *Service) CreateSnippet(ctx context.Context, content string, expiresIn int, tags []string) (domain.Snippet, error) {
	now := s.clock.Now()
	var expiresAt time.Time
	if expiresIn > 0 {
		expiresAt = now.Add(time.Duration(expiresIn) * time.Second)
	} else {
		expiresAt = time.Time{} // zero value, means no expiry
	}
	gen := s.idGen
	if gen == nil {
		gen = generateID
	}
	snippet := domain.Snippet{
		ID:        gen(),
		Content:   content,
		Tags:      tags,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}
	if err := s.repo.Insert(ctx, snippet); err != nil {
		return domain.Snippet{}, err
	}
	return snippet, nil
}

// ListSnippets returns a paginated list of snippets, optionally filtered by tag.
const (
	ServiceDefaultPage  = 1
	ServiceDefaultLimit = 20
	ServiceMaxLimit     = 100
)

// ListSnippets returns a list of snippets with pagination and optional tag filtering.
func (s *Service) ListSnippets(ctx context.Context, page, limit int, tag string) ([]domain.Snippet, error) {
	if limit > ServiceMaxLimit {
		limit = ServiceMaxLimit
	}
	if limit < 1 {
		limit = ServiceDefaultLimit
	}
	if page < 1 {
		page = ServiceDefaultPage
	}
	return s.repo.List(ctx, page, limit, tag)
}

// CacheStatus is a typed cache status string.
type CacheStatus string

const (
	// CacheMiss indicates a cache miss status.
	CacheMiss CacheStatus = "MISS"
	// CacheHit indicates a cache hit status.
	CacheHit CacheStatus = "HIT"
)

// SnippetMeta holds metadata about a snippet fetch.
type SnippetMeta struct {
	CacheStatus CacheStatus
}

// GetSnippetByID fetches a snippet by ID, returns metadata.
func (s *Service) GetSnippetByID(ctx context.Context, id string) (domain.Snippet, SnippetMeta, error) {
	// For demo, always MISS. Replace with real cache logic if needed.
	snippet, err := s.repo.FindByID(ctx, id)
	meta := SnippetMeta{CacheStatus: CacheMiss}
	if err != nil {
		// Only translate not found at the service boundary
		if errors.Is(err, repository.ErrNotFound) {
			return domain.Snippet{}, meta, fmt.Errorf("%w", ErrSnippetNotFound)
		}
		// All other errors are just wrapped
		return domain.Snippet{}, meta, fmt.Errorf("find by id: %w", err)
	}
	if !snippet.ExpiresAt.IsZero() && s.clock.Now().After(snippet.ExpiresAt) {
		return domain.Snippet{}, meta, fmt.Errorf("expired: %w", ErrSnippetExpired)
	}
	return snippet, meta, nil
}
