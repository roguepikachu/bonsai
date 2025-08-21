// Package main is the entry point for the Bonsai API server.
package main

import (
	"context"

	"github.com/roguepikachu/bonsai/internal/http/router"
	"github.com/roguepikachu/bonsai/pkg/logger"
)

func main() {
	router := router.Router()
	err := router.Run(":8080")
	if err != nil {
		logger.Fatal(context.Background(), "Failed to start server: %v", err)
	}
}
