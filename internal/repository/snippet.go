// Package repository defines repository interfaces for data access.
package repository

import (
	"context"

	"github.com/roguepikachu/bonsai/internal/domain"
)

// SnippetRepository defines methods for snippet data access.
type SnippetRepository interface {
	Insert(ctx context.Context, s domain.Snippet) (string, error)
	FindByID(ctx context.Context, id string) (domain.Snippet, error)
}
