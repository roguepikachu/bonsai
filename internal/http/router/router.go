// Package router sets up the HTTP routes for the Bonsai API server.
package router

import (
	"github.com/gin-gonic/gin"

	"github.com/roguepikachu/bonsai/internal/http/handler"
)

// Router initializes and returns the main Gin engine with all routes.
func Router() *gin.Engine {
	router := gin.Default()
	router.GET("/ping", handler.HealthCheck)
	return router
}
