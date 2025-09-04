// Package middleware provides HTTP middleware functions.
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	ctxutil "github.com/roguepikachu/bonsai/internal/utils"
)

const (
	headerRequestID = "X-Request-ID"
	headerClientID  = "X-Client-ID"
)

// RequestIDMiddleware sets a unique requestId in the context for each request.
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// RequestID: always set (generate if not provided)
		requestID := c.GetHeader(headerRequestID)
		if requestID == "" {
			requestID = uuid.New().String()
		}
		// ClientID: optional header, but fallback to UUID if missing for traceability
		clientID := c.GetHeader(headerClientID)
		if clientID == "" {
			clientID = uuid.New().String()
		}
		// Propagate via context and response headers
		ctx := ctxutil.WithRequestID(c.Request.Context(), requestID)
		ctx = ctxutil.WithClientID(ctx, clientID)
		c.Request = c.Request.WithContext(ctx)
		c.Header(headerRequestID, requestID)
		c.Header(headerClientID, clientID)
		c.Next()
	}
}
