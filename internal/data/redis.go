package data

import (
	"os"

	"github.com/go-redis/redis/v8"
)

// NewRedisClient creates and returns a new Redis client using environment variables.
func NewRedisClient() *redis.Client {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	return redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
}
