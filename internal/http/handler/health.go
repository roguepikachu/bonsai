// Package handler provides HTTP handler functions for the Bonsai API.
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/roguepikachu/bonsai/pkg"
)

// BonsaiHealthCheck handles the /ping endpoint for health checks.
func BonsaiHealthCheck(c *gin.Context) {
	response := pkg.NewResponse(http.StatusOK, nil, "pong")
	c.JSON(response.Code, response)
}
