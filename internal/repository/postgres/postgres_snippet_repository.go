// Package postgres provides a Postgres-backed implementation of the snippet repository.
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/roguepikachu/bonsai/internal/domain"
	"github.com/roguepikachu/bonsai/internal/repository"
)

// SnippetRepository implements repository.SnippetRepository using Postgres.
type SnippetRepository struct {
	pool *pgxpool.Pool
}

// NewSnippetRepository creates a new Postgres-backed snippet repository.
func NewSnippetRepository(pool *pgxpool.Pool) *SnippetRepository {
	return &SnippetRepository{pool: pool}
}


// Insert adds a new snippet to Postgres.
func (r *SnippetRepository) Insert(ctx context.Context, s domain.Snippet) error {
	var expires *time.Time
	if !s.ExpiresAt.IsZero() {
		expires = &s.ExpiresAt
	}
	tagsJSON, err := json.Marshal(s.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}
	const q = `
INSERT INTO snippets (id, content, tags, created_at, expires_at)
VALUES ($1, $2, $3::jsonb, $4, $5)
ON CONFLICT (id) DO NOTHING
`
	ct, err := r.pool.Exec(ctx, q, s.ID, s.Content, string(tagsJSON), s.CreatedAt, expires)
	if err != nil {
		return fmt.Errorf("insert snippet: %w", err)
	}
	if ct.RowsAffected() == 0 {
		// Treat as success for idempotency, or could return an error indicating duplicate.
		return nil
	}
	return nil
}

// FindByID retrieves a snippet by its ID from Postgres.
func (r *SnippetRepository) FindByID(ctx context.Context, id string) (domain.Snippet, error) {
	const q = `
SELECT id, content, tags, created_at, expires_at
FROM snippets
WHERE id = $1
`
	var (
		s          domain.Snippet
		tagsRaw    []byte
		expiresPtr *time.Time
	)
	err := r.pool.QueryRow(ctx, q, id).Scan(&s.ID, &s.Content, &tagsRaw, &s.CreatedAt, &expiresPtr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Snippet{}, repository.ErrNotFound
		}
		return domain.Snippet{}, fmt.Errorf("query snippet: %w", err)
	}
	if expiresPtr != nil {
		s.ExpiresAt = *expiresPtr
	}
	if len(tagsRaw) > 0 {
		if err := json.Unmarshal(tagsRaw, &s.Tags); err != nil {
			return domain.Snippet{}, fmt.Errorf("unmarshal tags: %w", err)
		}
	}
	return s, nil
}

// List returns a paginated list of snippets, optionally filtered by a tag. Excludes expired.
func (r *SnippetRepository) List(ctx context.Context, page, limit int, tag string) ([]domain.Snippet, error) {
	offset := (page - 1) * limit
	base := `
SELECT id, content, tags, created_at, expires_at
FROM snippets
WHERE (expires_at IS NULL OR expires_at > NOW())
`
	var rows pgx.Rows
	var err error
	if tag != "" {
		// tags @> '["tag"]'::jsonb
		q := base + " AND tags @> $1::jsonb ORDER BY created_at DESC LIMIT $2 OFFSET $3"
		tagJSON, _ := json.Marshal([]string{tag})
		rows, err = r.pool.Query(ctx, q, string(tagJSON), limit, offset)
	} else {
		q := base + " ORDER BY created_at DESC LIMIT $1 OFFSET $2"
		rows, err = r.pool.Query(ctx, q, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("list snippets: %w", err)
	}
	defer rows.Close()
	res := make([]domain.Snippet, 0, limit)
	for rows.Next() {
		var s domain.Snippet
		var tagsRaw []byte
		var expiresPtr *time.Time
		if err := rows.Scan(&s.ID, &s.Content, &tagsRaw, &s.CreatedAt, &expiresPtr); err != nil {
			return nil, fmt.Errorf("scan snippet: %w", err)
		}
		if expiresPtr != nil {
			s.ExpiresAt = *expiresPtr
		}
		if len(tagsRaw) > 0 {
			_ = json.Unmarshal(tagsRaw, &s.Tags)
		}
		res = append(res, s)
	}
	if rows.Err() != nil {
		return nil, rows.Err()
	}
	return res, nil
}

// Update modifies an existing snippet in Postgres.
func (r *SnippetRepository) Update(ctx context.Context, s domain.Snippet) error {
	var expires *time.Time
	if !s.ExpiresAt.IsZero() {
		expires = &s.ExpiresAt
	}
	tagsJSON, err := json.Marshal(s.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}
	const q = `
UPDATE snippets
SET content = $2, tags = $3::jsonb, expires_at = $4
WHERE id = $1
`
	ct, err := r.pool.Exec(ctx, q, s.ID, s.Content, string(tagsJSON), expires)
	if err != nil {
		return fmt.Errorf("update snippet: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

// Delete removes a snippet from Postgres by ID.
func (r *SnippetRepository) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM snippets WHERE id = $1`
	ct, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete snippet: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

var _ repository.SnippetRepository = (*SnippetRepository)(nil)
