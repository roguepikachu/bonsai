// Package middleware provides HTTP middleware functions.
package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roguepikachu/bonsai/pkg/logger"
)

// RequestLogger logs each HTTP request with useful context for debugging.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		if raw != "" {
			path = path + "?" + raw
		}
		route := c.FullPath()

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		size := c.Writer.Size()
		if size < 0 {
			size = 0
		}
		// Aggregate errors if any
		var errs string
		if len(c.Errors) > 0 {
			arr := make([]string, 0, len(c.Errors))
			for _, e := range c.Errors {
				arr = append(arr, e.Error())
			}
			errs = strings.Join(arr, "; ")
		}

		fields := map[string]any{
			"method":     c.Request.Method,
			"path":       path,
			"route":      route,
			"status":     status,
			"latency":    latency.String(),
			"latency_ms": latency.Milliseconds(),
			"bytes":      size,
			"ip":         c.ClientIP(),
			"ua":         c.Request.UserAgent(),
			"referer":    c.Request.Referer(),
		}
		if errs != "" {
			fields["errors"] = errs
		}

		entry := logger.With(c.Request.Context(), fields)
		switch {
		case status >= 500:
			entry.Error("request completed")
		case status >= 400:
			entry.Warn("request completed")
		default:
			entry.Info("request completed")
		}
	}
}
