// Package router sets up the HTTP routes for the Bonsai API server.
package router

// Router initializes and returns the main Gin engine with all routes.
import (
	"github.com/gin-gonic/gin"

	"github.com/roguepikachu/bonsai/internal/http/handler"
	"github.com/roguepikachu/bonsai/pkg/utils"
)

// Router initializes and returns the main Gin engine with all routes.
func Router() *gin.Engine {
	router := gin.Default()
	router.GET(utils.HealthCheckPath, handler.HealthCheck)
	return router
}
