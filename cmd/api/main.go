package main

import (
	"github.com/gin-gonic/gin"

	"github.com/roguepikachu/bonsai/internal/http/handler"
)

func main() {
	// Initialize the API server
	router := gin.Default()
	router.GET("/ping", handler.HealthCheck)

	router.Run("localhost:8080")
}
