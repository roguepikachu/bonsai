// Package data provides low-level data clients and connection factories.
package data

import (
	"github.com/go-redis/redis/v8"
	"github.com/roguepikachu/bonsai/internal/config"
)

// NewRedisClient creates and returns a new Redis client using environment variables.
func NewRedisClient() *redis.Client {
	redisAddr := config.Conf.RedisPort
	if redisAddr == "" {
		redisAddr = ":6379"
	}
	return redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
}
