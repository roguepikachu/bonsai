# Database Schema Migration Guide for Bonsai

This guide covers PostgreSQL schema migrations with Redis cache coordination for zero-downtime deployments.

## Table of Contents

- [Quick Start](#quick-start)
- [Migration Configuration](#migration-configuration)
- [Expand-Contract Migration Pattern](#expand-contract-migration-pattern)
- [Redis Cache Coordination](#redis-cache-coordination)
- [Kubernetes Deployment](#kubernetes-deployment)
- [CI/CD Integration](#cicd-integration)
- [Rollback Procedures](#rollback-procedures)
- [PgBouncer Considerations](#pgbouncer-considerations)
- [Troubleshooting](#troubleshooting)

## Quick Start

### Prerequisites

- Atlas CLI installed: `make migrate-install`
- PostgreSQL 12+ with JSONB support
- Redis 6+ for caching
- Kubernetes cluster for production deployment

### Basic Operations

```bash
# Check migration status
make migrate-status

# Generate migration from schema changes
make migrate-diff NAME=add_user_preferences

# Validate migration safety
make migrate-lint

# Apply migrations (dry run first)
make migrate-dry-run
make migrate-apply

# Check for schema drift
make migrate-drift
```

## Migration Configuration

The migration configuration is defined in `atlas.hcl`:

```hcl
env "production" {
  src = "file://schema"
  url = env("DATABASE_URL")
  migration {
    dir = "file://atlas-migrations"
  }
  diff {
    concurrent_index {
      create = true
      drop = true
    }
  }
  lint {
    destructive {
      error = true
    }
    data_depend {
      error = true
    }
    naming {
      error = true
    }
  }
}
```

### Schema Definition

Schema is defined declaratively in `schema/snippets.hcl`:

```hcl
table "snippets" {
  schema = schema.public

  column "id" {
    null = false
    type = text
  }

  column "content" {
    null = false
    type = text
  }

  # Indexes with concurrent creation
  index "idx_snippets_created_at" {
    columns = [column.created_at]
    type    = BTREE
    on {
      column = column.created_at
      desc   = true
    }
  }
}
```

## Expand-Contract Migration Pattern

The expand-contract pattern ensures zero-downtime schema changes by breaking changes into safe phases.

### Phase 1: Expand (Add New Schema)

**Goal**: Add new schema elements without removing existing ones.

```bash
# Example: Adding a new column
make migrate-diff NAME=add_snippet_language
```

Generated migration:
```sql
-- Add new column with default value
ALTER TABLE "public"."snippets" ADD COLUMN "language" text DEFAULT 'text';

-- Create index concurrently for new column
CREATE INDEX CONCURRENTLY "idx_snippets_language" ON "public"."snippets" ("language");
```

**Application Changes (Deploy 1)**:
```go
type Snippet struct {
    ID       string    `json:"id"`
    Content  string    `json:"content"`
    Language *string   `json:"language,omitempty"` // Optional during transition
    // ... other fields
}

// Repository methods handle both old and new schema
func (r *Repository) Insert(ctx context.Context, s Snippet) error {
    language := "text" // default value
    if s.Language != nil {
        language = *s.Language
    }

    const q = `INSERT INTO snippets (id, content, language, ...) VALUES ($1, $2, $3, ...)`
    _, err := r.db.Exec(ctx, q, s.ID, s.Content, language, ...)

    // Cache invalidation for both old and new patterns
    r.invalidateCache(ctx, s.ID, s.Language)
    return err
}
```

### Phase 2: Contract (Remove Old Schema)

**Goal**: Remove deprecated schema elements after ensuring no code dependencies.

Wait for:
1. ✅ All application instances are deployed with expand-compatible code
2. ✅ No queries use old columns/indexes
3. ✅ Cache warming period completed (typically 24-48 hours)

```bash
# Example: Removing old column after migration period
make migrate-diff NAME=remove_old_tags_format
```

Generated migration:
```sql
-- Drop old index first
DROP INDEX CONCURRENTLY "idx_snippets_old_format";

-- Remove old column
ALTER TABLE "public"."snippets" DROP COLUMN "old_tags_format";
```

**Application Changes (Deploy 2)**:
```go
type Snippet struct {
    ID       string `json:"id"`
    Content  string `json:"content"`
    Language string `json:"language"` // Now required
    // Removed: old_tags_format
}
```

### Phase 3: Optimize (Cleanup and Optimize)

```bash
make migrate-diff NAME=optimize_indexes_after_migration
```

## Redis Cache Coordination

Redis is used **only as a cache** with read-through, write-invalidate patterns.

### Cache Invalidation Strategy

```go
package cached

// Cache keys follow hierarchical pattern
const (
    keySnippet     = "bonsai:snippet:%s"           // Individual snippet
    keySnippetList = "bonsai:list:%s:%d:%d"        // Paginated lists
    keySnippetTag  = "bonsai:tag:%s:%d:%d"         // Tag-filtered lists
)

// Invalidation during schema changes
func (r *CachedRepository) InvalidateOnSchemaChange(ctx context.Context, migrationName string) error {
    switch migrationName {
    case "add_snippet_language":
        // Invalidate all list caches as they may need new fields
        return r.invalidatePattern(ctx, "bonsai:list:*")

    case "remove_old_tags_format":
        // Full cache clear for major schema changes
        return r.redis.FlushDB(ctx).Err()

    default:
        // Conservative: invalidate all lists but keep individual items
        return r.invalidatePattern(ctx, "bonsai:list:*")
    }
}

// Migration-aware cache warming
func (r *CachedRepository) WarmCachePostMigration(ctx context.Context) error {
    // Pre-populate frequently accessed data
    popularSnippets, err := r.primary.List(ctx, 1, 50, "")
    if err != nil {
        return err
    }

    for _, snippet := range popularSnippets {
        key := fmt.Sprintf(keySnippet, snippet.ID)
        data, _ := json.Marshal(snippet)
        r.redis.SetEX(ctx, key, string(data), r.ttl)
    }

    return nil
}
```

### Cache Coordination During Deployments

```yaml
# k8s/deployment.yaml - Rolling update strategy
spec:
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
      maxSurge: 1
  template:
    spec:
      initContainers:
      - name: atlas-migration
        # ... migration container
      - name: cache-warmup
        image: bonsai:latest
        command: ["/app/cache-warmup"]
        env:
        - name: REDIS_URL
          valueFrom:
            secretKeyRef:
              name: bonsai-redis
              key: url
```

## Kubernetes Deployment

### Option 1: Migration Job (Recommended)

Run migrations as a separate Kubernetes Job:

```bash
kubectl apply -f k8s/atlas-migration-job.yaml
kubectl wait --for=condition=complete job/bonsai-migration --timeout=300s
kubectl apply -f k8s/bonsai-deployment.yaml
```

### Option 2: Init Container

Migrations run automatically before each pod starts:

```bash
kubectl apply -f k8s/bonsai-deployment-with-migration.yaml
```

### Migration Job Features

- **Advisory Locking**: Built-in locking prevents concurrent migrations
- **Dry Run**: Always runs dry-run before applying changes
- **Security**: Runs as non-root with minimal permissions
- **Monitoring**: Proper logging and exit codes for CI/CD integration

## CI/CD Integration

### GitHub Actions Workflow

The CI pipeline (`/.github/workflows/atlas-drift-detection.yml`) provides:

1. **Schema Validation**: Validates migration configuration and schema files
2. **Migration Linting**: Checks for destructive or unsafe operations
3. **Drift Detection**: Compares desired schema with live database
4. **Safety Checks**: Ensures concurrent index operations and PgBouncer compatibility

### Drift Detection

```bash
# Manual drift check
make migrate-drift

# In CI - fails build on drift
atlas schema diff --env production --from "$DATABASE_URL" --to file://schema
```

If drift is detected:
1. Review the differences carefully
2. Run `make migrate-diff NAME=fix_drift` to generate corrective migration
3. Test migration in staging environment
4. Apply to production during maintenance window

## Rollback Procedures

### Emergency Rollback Process

Automatic rollback is not supported, so follow these procedures:

#### 1. Application Rollback (Quick)

```bash
# Rollback application to previous version
kubectl rollout undo deployment/bonsai-api

# Clear caches to prevent stale data
kubectl exec deployment/bonsai-api -- /app/cache-clear-all
```

#### 2. Schema Rollback (If Required)

Create reverse migration manually:

```bash
# Generate reverse migration
make migrate-diff NAME=rollback_problematic_change

# Review generated rollback migration
cat atlas-migrations/$(date +%Y%m%d%H%M%S)_rollback_problematic_change.sql

# Apply after testing
make migrate-apply ATLAS_ENV=production
```

#### 3. Data Recovery (Last Resort)

```bash
# Restore from backup if data loss occurred
pg_restore --clean --no-owner --no-acl --dbname="$DATABASE_URL" latest_backup.dump

# Rebuild caches
kubectl exec deployment/bonsai-api -- /app/cache-warmup
```

### Rollback Prevention

1. **Always test in staging** with production-like data
2. **Use feature flags** for application-level changes
3. **Monitor key metrics** during deployment
4. **Have automated rollback triggers** based on error rates

## PgBouncer Considerations

### Transaction vs Session Pooling

**Problem**: Some operations require session-level connections.

```sql
-- These operations require session connections:
CREATE INDEX CONCURRENTLY idx_name ON table (column);
DROP INDEX CONCURRENTLY idx_name;
```

**Solution**: Direct database connection for migrations.

```yaml
# k8s/atlas-migration-job.yaml
env:
- name: DATABASE_URL
  value: "postgres://user:pass@postgres-primary:5432/db?sslmode=disable"
  # NOT through PgBouncer transaction pool
```

### Application Connection Strategy

```go
// config/database.go
type DatabaseConfig struct {
    // Application queries through PgBouncer (transaction mode)
    AppURL      string `env:"DATABASE_URL"`

    // Migrations through direct connection (session mode)
    MigrationURL string `env:"MIGRATION_DATABASE_URL"`
}

// Migration tool uses session-preserving connection
func RunMigrations() error {
    db, err := sql.Open("postgres", config.MigrationURL)
    // ... run migrations
}
```

## Sample Commands and Workflows

### Development Workflow

```bash
# 1. Modify schema in schema/snippets.hcl
vim schema/snippets.hcl

# 2. Generate migration
make migrate-diff NAME=add_snippet_categories

# 3. Review generated migration
cat atlas-migrations/$(ls -t atlas-migrations/*.sql | head -1)

# 4. Test migration locally
make migrate-apply ATLAS_ENV=local

# 5. Run application tests
make test-integration

# 6. Commit changes
git add schema/ atlas-migrations/
git commit -m "feat: add snippet categories support"
```

### Production Deployment

```bash
# 1. Deploy migration job
kubectl apply -f k8s/atlas-migration-job.yaml

# 2. Wait for completion
kubectl wait --for=condition=complete job/bonsai-migration --timeout=300s

# 3. Check migration status
kubectl logs job/bonsai-migration

# 4. Deploy application (expand-compatible version)
kubectl set image deployment/bonsai-api bonsai-api=bonsai:v1.2.0

# 5. Monitor metrics and health checks
kubectl get pods -l app=bonsai
curl http://bonsai-api/v1/healthz

# 6. After stabilization period, deploy contract changes
# (Usually in next deployment cycle)
```

### Monitoring and Alerting

```bash
# Check migration status
make migrate-status

# Monitor database performance impact
kubectl exec postgres-0 -- psql -c "
SELECT
    schemaname,
    tablename,
    indexname,
    idx_tup_read,
    idx_tup_fetch
FROM pg_stat_user_indexes
WHERE indexname LIKE 'idx_snippets_%'
ORDER BY idx_tup_read DESC;
"

# Monitor cache hit rates
kubectl exec redis-0 -- redis-cli INFO stats | grep keyspace
```

## Troubleshooting

### Common Issues

#### 1. Migration Lock Timeout

```bash
# Check for long-running migrations
kubectl logs job/bonsai-migration

# Manual unlock (emergency only)
kubectl exec postgres-0 -- psql -c "
SELECT pg_advisory_unlock_all();
SELECT * FROM atlas_schema_revisions;
"
```

#### 2. Schema Drift in Production

```bash
# Immediate assessment
make migrate-drift ATLAS_ENV=production

# Generate corrective migration
make migrate-diff NAME=fix_production_drift

# Emergency schema inspection
make migrate-inspect ATLAS_ENV=production
```

#### 3. Cache Inconsistency After Migration

```bash
# Full cache clear
kubectl exec deployment/bonsai-api -- /app/cache-clear-all

# Restart Redis (if safe)
kubectl rollout restart statefulset/redis

# Warm cache with new schema
kubectl exec deployment/bonsai-api -- /app/cache-warmup
```

#### 4. PgBouncer Connection Issues

```bash
# Check PgBouncer status
kubectl exec pgbouncer-0 -- psql -p 6432 pgbouncer -c "SHOW POOLS;"

# Verify migration connection bypasses pool
kubectl logs job/bonsai-migration | grep "connection"
```

### Debug Commands

```bash
# Migration debug mode
atlas migrate apply --env production --dry-run --log-level debug

# Database connection test
kubectl run -it --rm debug --image=postgres:16 --restart=Never -- \
    psql "$DATABASE_URL" -c "SELECT version();"

# Cache connection test
kubectl run -it --rm debug --image=redis:7 --restart=Never -- \
    redis-cli -u "$REDIS_URL" ping
```

### Performance Monitoring

```sql
-- Monitor concurrent index creation progress
SELECT
    pid,
    state,
    query_start,
    query
FROM pg_stat_activity
WHERE query LIKE '%CREATE INDEX CONCURRENTLY%';

-- Check table bloat after migrations
SELECT
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size,
    pg_size_pretty(pg_relation_size(schemaname||'.'||tablename)) as table_size,
    pg_size_pretty(pg_indexes_size(schemaname||'.'||tablename)) as index_size
FROM pg_tables
WHERE tablename = 'snippets';
```

## Best Practices Summary

1. **Always use expand-contract pattern** for breaking changes
2. **Test migrations in staging** with production-like data volume
3. **Use concurrent index operations** for zero-downtime deployments
4. **Monitor database performance** during and after migrations
5. **Coordinate cache invalidation** with schema changes
6. **Use session connections for migrations**, not transaction pools
7. **Have rollback procedures ready** before applying changes
8. **Run drift detection regularly** to catch manual schema changes
9. **Use feature flags** to decouple deployments from feature rollouts
10. **Monitor cache hit rates** and application performance post-migration

For additional help, see the [Atlas documentation](https://atlasgo.io/guides) or check the troubleshooting section above.