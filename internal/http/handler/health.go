// Package handler provides HTTP handler functions for the Bonsai API.
package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roguepikachu/bonsai/pkg"
	"github.com/roguepikachu/bonsai/pkg/logger"
)

// Health keeps the legacy simple health endpoint for backwards compatibility.
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, pkg.NewResponse(http.StatusOK, gin.H{"ok": true}, "ok"))
}

// HealthHandler provides liveness and readiness probes checking downstream deps.
type Pinger interface {
	Ping(ctx context.Context) error
}

type HealthHandler struct {
	pg    Pinger
	redis Pinger
	// optional: future deps can be added here
	pingTimeout time.Duration
}

// NewHealthHandler constructs a HealthHandler.
func NewHealthHandler(pg *pgxpool.Pool, redis *redis.Client) *HealthHandler {
	// Adapters turning concrete clients into Pinger
	var pgPinger Pinger
	if pg != nil {
		pgPinger = pgPingerAdapter{pg}
	}
	var redisPinger Pinger
	if redis != nil {
		redisPinger = redisPingerAdapter{redis}
	}
	return &HealthHandler{
		pg:          pgPinger,
		redis:       redisPinger,
		pingTimeout: 1 * time.Second,
	}
}

type pgPingerAdapter struct{ pool *pgxpool.Pool }

func (p pgPingerAdapter) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }

type redisPingerAdapter struct{ c *redis.Client }

func (r redisPingerAdapter) Ping(ctx context.Context) error { return r.c.Ping(ctx).Err() }

// Liveness reports that the process is up. Do not check external deps here.
func (h *HealthHandler) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, pkg.NewResponse(http.StatusOK, gin.H{"status": "alive"}, "ok"))
}

// Readiness checks external dependencies to decide if we can serve traffic.
func (h *HealthHandler) Readiness(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.pingTimeout)
	defer cancel()

	type check struct {
		name   string
		status string
		err    string
	}
	results := make([]check, 0, 2)
	ready := true

	// Postgres
	if h.pg != nil {
		if err := h.pg.Ping(ctx); err != nil {
			ready = false
			results = append(results, check{name: "postgres", status: "down", err: err.Error()})
		} else {
			results = append(results, check{name: "postgres", status: "up"})
		}
	}

	// Redis
	if h.redis != nil {
		if err := h.redis.Ping(ctx); err != nil {
			ready = false
			results = append(results, check{name: "redis", status: "down", err: err.Error()})
		} else {
			results = append(results, check{name: "redis", status: "up"})
		}
	}

	if ready {
		c.JSON(http.StatusOK, pkg.NewResponse(http.StatusOK, gin.H{"ready": true, "checks": results}, "ready"))
		return
	}
	logger.Warn(c.Request.Context(), "readiness failed: %+v", results)
	c.JSON(http.StatusServiceUnavailable, pkg.NewResponse(http.StatusServiceUnavailable, gin.H{"ready": false, "checks": results}, "not ready"))
}
