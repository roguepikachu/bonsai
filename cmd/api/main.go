// Package main is the entry point for the Bonsai API server.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	healthHandler := handler.NewHealthHandler(pgPool, redisClient)

	r := appRouter.NewRouter(snippetHandler, healthHandler)

	port := config.Conf.BonsaiPort
	if port == "" {
		logger.Info(ctx, "no port configured, falling back to default: 8080")
		port = "8080"
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Start server in background
	go func() {
		logger.WithField(ctx, "addr", ":"+port).Info("starting server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal(ctx, "server error: %v", err)
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	logger.WithField(ctx, "signal", "interrupt").Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.WithField(ctx, "error", err.Error()).Error("graceful shutdown failed")
		if cerr := srv.Close(); cerr != nil {
			logger.WithField(ctx, "error", cerr.Error()).Error("server close failed")
		}
	}
	logger.Info(ctx, "server stopped cleanly")
}
