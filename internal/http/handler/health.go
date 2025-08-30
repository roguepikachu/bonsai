// Package handler provides HTTP handler functions for the Bonsai API.
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/roguepikachu/bonsai/pkg"
)

// Health handles the /health endpoint for health checks.
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, pkg.NewResponse(http.StatusOK, gin.H{"ok": true}, "ok"))
}
