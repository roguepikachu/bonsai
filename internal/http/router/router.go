// Package router sets up the HTTP routes for the Bonsai API server.
package router

// Router initializes and returns the main Gin engine with all routes.
import (
	"github.com/gin-gonic/gin"

	"github.com/roguepikachu/bonsai/internal/http/handler"
)

const (
	// BasePath is the root path for the API.
	BasePath = "/api/v1"

	// BonsaiServiceHealth is the endpoint for health checks.
	BonsaiServiceHealth = BasePath + "/ping"
)

// NewRouter initializes and returns the main Gin engine with all routes.
func NewRouter() *gin.Engine {
	router := gin.Default()
	router.GET(BonsaiServiceHealth, handler.BonsaiHealthCheck)
	return router
}
