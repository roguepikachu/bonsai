// Package redis provides a Redis-backed implementation of the snippet repository.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/roguepikachu/bonsai/internal/domain"
	"github.com/roguepikachu/bonsai/internal/repository"
)

// SnippetRepository implements repository.SnippetRepository using Redis as backend.
type SnippetRepository struct {
	client *redis.Client
}

// NewSnippetRepository creates a new Redis-backed snippet repository.
func NewSnippetRepository(client *redis.Client) *SnippetRepository {
	return &SnippetRepository{client: client}
}

// Insert adds a new snippet to Redis.
func (r *SnippetRepository) Insert(ctx context.Context, s domain.Snippet) error {
	key := fmt.Sprintf("snippet:%s", s.ID)
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	expiry := time.Until(s.ExpiresAt)
	if s.ExpiresAt.IsZero() {
		expiry = 0
	}
	if err := r.client.Set(ctx, key, data, expiry).Err(); err != nil {
		return err
	}
	return nil
}

// FindByID retrieves a snippet by its ID from Redis.
func (r *SnippetRepository) FindByID(ctx context.Context, id string) (domain.Snippet, error) {
	key := fmt.Sprintf("snippet:%s", id)
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return domain.Snippet{}, fmt.Errorf("redis get: %w", err)
	}
	var s domain.Snippet
	if err := json.Unmarshal([]byte(val), &s); err != nil {
		return domain.Snippet{}, fmt.Errorf("unmarshal: %w", err)
	}
	return s, nil
}

// List returns a paginated list of snippets, optionally filtered by tag.
func (r *SnippetRepository) List(ctx context.Context, page, limit int, tag string) ([]domain.Snippet, error) {
	var snippets []domain.Snippet
	var cursor uint64
	pattern := "snippet:*"
	for {
		keys, next, err := r.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			val, err := r.client.Get(ctx, key).Result()
			if err != nil {
				continue
			}
			var s domain.Snippet
			if err := json.Unmarshal([]byte(val), &s); err != nil {
				continue
			}
			if tag != "" {
				found := false
				for _, t := range s.Tags {
					if t == tag {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			snippets = append(snippets, s)
		}
		if next == 0 {
			break
		}
		cursor = next
	}
	// Sort by CreatedAt descending
	sort.Slice(snippets, func(i, j int) bool {
		return snippets[i].CreatedAt.After(snippets[j].CreatedAt)
	})
	start := (page - 1) * limit
	if start > len(snippets) {
		return []domain.Snippet{}, nil
	}
	end := start + limit
	if end > len(snippets) {
		end = len(snippets)
	}
	return snippets[start:end], nil
}

var _ repository.SnippetRepository = (*SnippetRepository)(nil)
