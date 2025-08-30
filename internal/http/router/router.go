// Package router sets up the HTTP routes for the Bonsai API server.
package router

import (
	"github.com/gin-gonic/gin"

	"github.com/roguepikachu/bonsai/internal/http/handler"
	"github.com/roguepikachu/bonsai/internal/http/middleware"
)

const (
	// BasePath is the root path for the API.
	BasePath = "/v1"

	// HealthPath is the legacy endpoint for health checks.
	HealthPath = BasePath + "/health"
	// LivenessPath returns 200 when process is running.
	LivenessPath = BasePath + "/livez"
	// ReadinessPath checks dependencies and returns 200/503 accordingly.
	ReadinessPath = BasePath + "/readyz"
)

// NewRouter initializes and returns the main Gin engine with all routes.
func NewRouter(snippetHandler *handler.Handler, healthHandler *handler.HealthHandler) *gin.Engine {
	router := gin.Default()
	router.Use(middleware.RequestIDMiddleware())
	// Legacy health
	router.GET(HealthPath, handler.Health)
	// Kubernetes-style probes
	if healthHandler != nil {
		router.GET(LivenessPath, healthHandler.Liveness)
		router.GET(ReadinessPath, healthHandler.Readiness)
	}

	router.POST(BasePath+"/snippets", snippetHandler.Create)
	router.GET(BasePath+"/snippets", snippetHandler.List)
	router.GET(BasePath+"/snippets/:id", snippetHandler.Get)

	return router
}
