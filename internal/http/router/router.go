// Package router sets up the HTTP routes for the Bonsai API server.
package router

// Router initializes and returns the main Gin engine with all routes.
import (
	"github.com/gin-gonic/gin"

	"github.com/roguepikachu/bonsai/internal/data"
	"github.com/roguepikachu/bonsai/internal/http/handler"
	redisrepo "github.com/roguepikachu/bonsai/internal/repository/redis"
	"github.com/roguepikachu/bonsai/internal/service"
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

	// Setup Redis client
	redisClient := data.NewRedisClient()

	// Setup repository and service
	repo := redisrepo.NewRedisSnippetRepository(redisClient)
	svc := service.NewService(repo)
	snippetHandler := &handler.Handler{Svc: svc}
	router.POST(BasePath+"/snippets", snippetHandler.Create)

	return router
}
