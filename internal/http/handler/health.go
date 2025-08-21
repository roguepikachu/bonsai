// Package handler provides HTTP handler functions for the Bonsai API.
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/roguepikachu/bonsai/pkg"
)

// HealthCheck handles the /ping endpoint for health checks.
func HealthCheck(c *gin.Context) {
	response := pkg.NewResponse(http.StatusOK, nil, "pong")
	c.JSON(response.Code, response)
}
