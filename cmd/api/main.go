// Package main is the entry point for the Bonsai API server.
package main

import (
	"context"

	"github.com/roguepikachu/bonsai/internal/config"
	"github.com/roguepikachu/bonsai/internal/http/router"
	"github.com/roguepikachu/bonsai/pkg/logger"
)

func main() {
	ctx := context.Background()

	router := router.NewRouter()

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
