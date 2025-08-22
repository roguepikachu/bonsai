// Package router sets up the HTTP routes for the Bonsai API server.
package router

// Router initializes and returns the main Gin engine with all routes.
import (
	"github.com/gin-gonic/gin"

	"github.com/roguepikachu/bonsai/internal/http/handler"
	"github.com/roguepikachu/bonsai/internal/http/middleware"
)

const (
	// BasePath is the root path for the API.
	BasePath = "/api/v1"

	// BonsaiServiceHealth is the endpoint for health checks.
	BonsaiServiceHealth = BasePath + "/ping"
)

// NewRouter initializes and returns the main Gin engine with all routes.
func NewRouter(snippetHandler *handler.Handler) *gin.Engine {
	router := gin.Default()
	router.Use(middleware.RequestIDMiddleware())
	router.GET(BonsaiServiceHealth, handler.BonsaiHealthCheck)

	router.POST(BasePath+"/snippets", snippetHandler.Create)
	router.GET(BasePath+"/snippets", snippetHandler.List)
	router.GET(BasePath+"/snippets/:id", snippetHandler.Get)

	return router
}
