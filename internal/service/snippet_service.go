// Package service contains business logic for the application.
package service

import (
	"context"
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
