// Package main is the entry point for the Bonsai API server.
package main

import (
	"context"

	"github.com/roguepikachu/bonsai/internal/config"
	"github.com/roguepikachu/bonsai/internal/data"
	"github.com/roguepikachu/bonsai/internal/http/handler"
	"github.com/roguepikachu/bonsai/internal/http/router"
	"github.com/roguepikachu/bonsai/internal/service"
	"github.com/roguepikachu/bonsai/pkg/logger"

	redisrepo "github.com/roguepikachu/bonsai/internal/repository/redis"
)

func init() {
	logger.InitLogging()
	config.InitConf()
}

func main() {
	ctx := context.Background()

	// Setup Redis client
	redisClient := data.NewRedisClient()

	// Setup repository and service
	repo := redisrepo.NewSnippetRepository(redisClient)
	svc := service.NewService(repo)
	snippetHandler := &handler.Handler{Svc: svc}

	router := router.NewRouter(snippetHandler)

	port := config.Conf.BonsaiPort
	if port == "" {
		logger.Info(ctx, "no port configured, falling back to default: 8080")
		port = "8080"
	}

	err := router.Run(":" + port)
	if err != nil {
		logger.Fatal(ctx, "failed to start server: %v", err)
	}
}
