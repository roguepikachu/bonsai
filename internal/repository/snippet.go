// SnippetRepository defines methods for snippet data access.
package repository

import (
	"context"

	"github.com/roguepikachu/bonsai/internal/domain"
)

type SnippetRepository interface {
	Insert(ctx context.Context, s domain.Snippet) (string, error)
	FindByID(ctx context.Context, id string) (domain.Snippet, error)
}
