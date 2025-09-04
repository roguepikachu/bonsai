// Package cached provides a caching wrapper over a primary repository using Redis.
package cached

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/roguepikachu/bonsai/internal/domain"
	"github.com/roguepikachu/bonsai/internal/repository"
	"github.com/roguepikachu/bonsai/pkg/logger"
)

// key helpers
func keySnippet(id string) string { return "snippet:" + id }
func keyList(page, limit int, tag string) string {
	if tag != "" {
		return fmt.Sprintf("snippets:p%d:l%d:t:%s", page, limit, tag)
	}
	return fmt.Sprintf("snippets:p%d:l%d", page, limit)
}

// SnippetRepository is a cache-aside repository combining Redis with a primary store.
type SnippetRepository struct {
	primary repository.SnippetRepository
	redis   *redis.Client
	ttl     time.Duration
}

// NewSnippetRepository creates a new cached repository.
func NewSnippetRepository(primary repository.SnippetRepository, redis *redis.Client, ttl time.Duration) *SnippetRepository {
	return &SnippetRepository{primary: primary, redis: redis, ttl: ttl}
}

// Insert writes through to primary and populates cache.
func (r *SnippetRepository) Insert(ctx context.Context, s domain.Snippet) error {
	if err := r.primary.Insert(ctx, s); err != nil {
		return err
	}
	// cache the snippet
	data, _ := json.Marshal(s)
	exp := r.ttl
	if !s.ExpiresAt.IsZero() {
		if until := time.Until(s.ExpiresAt); until > 0 && (exp == 0 || until < exp) {
			exp = until
		}
	}
	if err := r.redis.Set(ctx, keySnippet(s.ID), data, exp).Err(); err != nil {
		logger.With(ctx, map[string]any{"id": s.ID, "ttl": exp.String()}).Warn("failed to set snippet in cache")
	} else {
		logger.With(ctx, map[string]any{"id": s.ID, "ttl": exp.String()}).Debug("cached snippet after insert")
	}
	// bust list caches best-effort
	if err := r.invalidateListKeys(ctx); err != nil {
		logger.With(ctx, map[string]any{"error": err.Error()}).Warn("failed to invalidate list cache keys")
	}
	return nil
}

// FindByID attempts Redis then falls back to primary.
func (r *SnippetRepository) FindByID(ctx context.Context, id string) (domain.Snippet, error) {
	val, err := r.redis.Get(ctx, keySnippet(id)).Result()
	if err == nil && val != "" {
		var s domain.Snippet
		if jsonErr := json.Unmarshal([]byte(val), &s); jsonErr == nil {
			logger.WithField(ctx, "id", id).Debug("cache hit: snippet")
			return s, nil
		}
	}
	logger.WithField(ctx, "id", id).Debug("cache miss: snippet")
	s, err := r.primary.FindByID(ctx, id)
	if err != nil {
		return domain.Snippet{}, err
	}
	data, _ := json.Marshal(s)
	exp := r.ttl
	if !s.ExpiresAt.IsZero() {
		if until := time.Until(s.ExpiresAt); until > 0 && (exp == 0 || until < exp) {
			exp = until
		}
	}
	if err := r.redis.Set(ctx, keySnippet(s.ID), data, exp).Err(); err != nil {
		logger.With(ctx, map[string]any{"id": s.ID, "ttl": exp.String()}).Warn("failed to set snippet in cache")
	}
	return s, nil
}

// List caches the page results keyed by page/limit/tag.
func (r *SnippetRepository) List(ctx context.Context, page, limit int, tag string) ([]domain.Snippet, error) {
	k := keyList(page, limit, tag)
	if val, err := r.redis.Get(ctx, k).Result(); err == nil && val != "" {
		var items []domain.Snippet
		if jsonErr := json.Unmarshal([]byte(val), &items); jsonErr == nil {
			logger.With(ctx, map[string]any{"key": k}).Debug("cache hit: list")
			return items, nil
		}
	}
	logger.With(ctx, map[string]any{"key": k}).Debug("cache miss: list")
	items, err := r.primary.List(ctx, page, limit, tag)
	if err != nil {
		return nil, err
	}
	// eliminate already expired ones just in case
	now := time.Now()
	filtered := items[:0]
	for _, s := range items {
		if s.ExpiresAt.IsZero() || now.Before(s.ExpiresAt) {
			filtered = append(filtered, s)
		}
	}
	// ensure order by CreatedAt desc (primary should already do this)
	sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].CreatedAt.After(filtered[j].CreatedAt) })
	data, _ := json.Marshal(filtered)
	if err := r.redis.Set(ctx, k, data, r.ttl).Err(); err != nil {
		logger.With(ctx, map[string]any{"key": k, "ttl": r.ttl.String()}).Warn("failed to set list in cache")
	}
	return filtered, nil
}

func (r *SnippetRepository) invalidateListKeys(ctx context.Context) error {
	// scan-and-delete keys with prefix snippets:
	var cursor uint64
	for {
		keys, next, err := r.redis.Scan(ctx, cursor, "snippets:*", 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			// filter only list keys
			listKeys := make([]string, 0, len(keys))
			for _, k := range keys {
				if strings.HasPrefix(k, "snippets:") && !strings.HasPrefix(k, "snippet:") {
					listKeys = append(listKeys, k)
				}
			}
			if len(listKeys) > 0 {
				if err := r.redis.Del(ctx, listKeys...).Err(); err != nil {
					logger.With(ctx, map[string]any{"keys": listKeys, "error": err.Error()}).Warn("failed to delete list cache keys")
				} else {
					logger.With(ctx, map[string]any{"keys": listKeys}).Debug("invalidated list cache keys")
				}
			}
		}
		if next == 0 {
			break
		}
		cursor = next
	}
	return nil
}

// Update writes through to primary and invalidates cache.
func (r *SnippetRepository) Update(ctx context.Context, s domain.Snippet) error {
	if err := r.primary.Update(ctx, s); err != nil {
		return err
	}
	// invalidate the cached snippet
	if err := r.redis.Del(ctx, keySnippet(s.ID)).Err(); err != nil {
		logger.With(ctx, map[string]any{"id": s.ID}).Warn("failed to delete snippet from cache")
	} else {
		logger.With(ctx, map[string]any{"id": s.ID}).Debug("invalidated cached snippet after update")
	}
	// bust list caches best-effort
	if err := r.invalidateListKeys(ctx); err != nil {
		logger.With(ctx, map[string]any{"error": err.Error()}).Warn("failed to invalidate list cache keys")
	}
	return nil
}

var _ repository.SnippetRepository = (*SnippetRepository)(nil)
