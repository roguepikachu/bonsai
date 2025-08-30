// Package main is the entry point for the Bonsai API server.
package main

import (
	"context"
	"time"

	"github.com/roguepikachu/bonsai/internal/config"
	"github.com/roguepikachu/bonsai/internal/data"
	"github.com/roguepikachu/bonsai/internal/http/handler"
	appRouter "github.com/roguepikachu/bonsai/internal/http/router"
	"github.com/roguepikachu/bonsai/internal/service"
	"github.com/roguepikachu/bonsai/pkg/logger"

	cachedrepo "github.com/roguepikachu/bonsai/internal/repository/cached"
	pgrepo "github.com/roguepikachu/bonsai/internal/repository/postgres"
)

func init() {
	logger.InitLogging()
	config.InitConf()
}

func main() {
	ctx := context.Background()

	// Setup Redis client
	redisClient := data.NewRedisClient()
	defer redisClient.Close()

	// Setup Postgres pool
	pgPool, err := data.NewPostgresPool(ctx)
	if err != nil {
		logger.Fatal(ctx, "failed to init postgres: %v", err)
	}
	// Setup Postgres repository and ensure schema if configured
	pgRepo := pgrepo.NewSnippetRepository(pgPool)
	defer pgPool.Close()
	if config.Conf.AutoMigrate {
		if err := pgRepo.EnsureSchema(ctx); err != nil {
			logger.Fatal(ctx, "failed to ensure postgres schema: %v", err)
		}
	}

	// Compose cached repository: Postgres primary + Redis cache
	repo := cachedrepo.NewSnippetRepository(pgRepo, redisClient, 10*time.Minute)
	svc := service.NewService(repo, &service.RealClock{})
	snippetHandler := handler.NewHandler(svc)

	r := appRouter.NewRouter(snippetHandler)

	port := config.Conf.BonsaiPort
	if port == "" {
		logger.Info(ctx, "no port configured, falling back to default: 8080")
		port = "8080"
	}

	err = r.Run(":" + port)
	if err != nil {
		logger.Fatal(ctx, "failed to start server: %v", err)
	}
}
