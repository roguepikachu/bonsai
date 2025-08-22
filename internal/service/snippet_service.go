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

// NewService creates a new Service with the given SnippetRepository.
func NewService(repo repository.SnippetRepository) *Service {
	return &Service{Repo: repo}
}

// Service provides snippet-related business logic.
type Service struct {
	Repo repository.SnippetRepository
}

// Error variables
var (
	ErrSnippetNotFound = errors.New("snippet not found")
	ErrSnippetExpired  = errors.New("snippet expired")
)

// generateID returns a new unique ID for a snippet.
func generateID() string {
	return uuid.New().String()
}

// CreateSnippet creates a new snippet with content, expiry, and tags.
func (s *Service) CreateSnippet(ctx context.Context, content string, expiresIn int, tags []string) (domain.Snippet, error) {
	now := time.Now().UTC()
	var expiresAt time.Time
	if expiresIn > 0 {
		expiresAt = now.Add(time.Duration(expiresIn) * time.Second)
	} else {
		expiresAt = time.Time{} // zero value, means no expiry
	}
	snippet := domain.Snippet{
		ID:        generateID(), // implement or use a UUID generator
		Content:   content,
		Tags:      tags,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}
	_, err := s.Repo.Insert(ctx, snippet)
	if err != nil {
		return domain.Snippet{}, err
	}
	return snippet, nil
}

// ListSnippets returns a paginated list of snippets, optionally filtered by tag.
func (s *Service) ListSnippets(ctx context.Context, page, limit int, tag string) ([]domain.Snippet, error) {
	return s.Repo.List(ctx, page, limit, tag)
}

// GetSnippetByID fetches a snippet by ID, returns cache status ("HIT" or "MISS").
func (s *Service) GetSnippetByID(ctx context.Context, id string) (domain.Snippet, string, error) {
	// For demo, always MISS. Replace with real cache logic if needed.
	snippet, err := s.Repo.FindByID(ctx, id)
	if err != nil {
		return domain.Snippet{}, "MISS", fmt.Errorf("find by id: %w", ErrSnippetNotFound)
	}
	if !snippet.ExpiresAt.IsZero() && time.Now().UTC().After(snippet.ExpiresAt) {
		return domain.Snippet{}, "MISS", fmt.Errorf("expired: %w", ErrSnippetExpired)
	}
	return snippet, "MISS", nil
}
