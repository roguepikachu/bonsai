package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/roguepikachu/bonsai/pkg/logger"
)

// Recovery recovers from panics, logs them, and returns 500.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				// capture stack trace and panic value, but do not leak sensitive info to client
				logger.With(c.Request.Context(), map[string]any{"panic": r, "stack": string(debug.Stack())}).Error("panic recovered")
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": gin.H{"code": "internal_error", "message": "internal server error"},
				})
			}
		}()
		c.Next()
	}
}
