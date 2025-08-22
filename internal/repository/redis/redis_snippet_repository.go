// Package redis provides a Redis-backed implementation of the snippet repository.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
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
func (r *SnippetRepository) Insert(ctx context.Context, s domain.Snippet) (string, error) {
	key := fmt.Sprintf("snippet:%s", s.ID)
	data, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	expiry := time.Until(s.ExpiresAt)
	if s.ExpiresAt.IsZero() {
		expiry = 0
	}
	if err := r.client.Set(ctx, key, data, expiry).Err(); err != nil {
		return "", err
	}
	return s.ID, nil
}

// FindByID retrieves a snippet by its ID from Redis.
func (r *SnippetRepository) FindByID(ctx context.Context, id string) (domain.Snippet, error) {
	key := fmt.Sprintf("snippet:%s", id)
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return domain.Snippet{}, err
	}
	var s domain.Snippet
	if err := json.Unmarshal([]byte(val), &s); err != nil {
		return domain.Snippet{}, err
	}
	return s, nil
}

var _ repository.SnippetRepository = (*SnippetRepository)(nil)
