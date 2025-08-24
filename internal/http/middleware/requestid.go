// Package middleware provides HTTP middleware functions.
package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDMiddleware sets a unique requestId in the context for each request.

type ctxKeyRequestID struct{}

func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := uuid.New().String()
		c.Set("requestId", requestID)
		c.Request = c.Request.WithContext(
			context.WithValue(c.Request.Context(), ctxKeyRequestID{}, requestID),
		)
		c.Next()
	}
}
