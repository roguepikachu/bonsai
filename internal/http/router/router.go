package router

import (
	"github.com/gin-gonic/gin"

	"github.com/roguepikachu/bonsai/internal/http/handler"
)

func Router() *gin.Engine {
	router := gin.Default()
	router.GET("/ping", handler.HealthCheck)
	return router
}
