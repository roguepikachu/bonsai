// Package main is the entry point for the Bonsai API server.
package main

import (
	"github.com/sirupsen/logrus"

	"github.com/roguepikachu/bonsai/internal/http/router"
)

func main() {
	router := router.Router()
	err := router.Run("localhost:8080")
	if err != nil {
		logrus.Fatalf("Failed to start server: %v", err)
	}
}
