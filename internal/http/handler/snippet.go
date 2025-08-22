// Package handler provides HTTP handlers for the API endpoints.
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/roguepikachu/bonsai/internal/service"
	"github.com/roguepikachu/bonsai/pkg/logger"
)

// Handler handles HTTP requests for snippets.
type Handler struct {
	Svc *service.Service
}

// CreateSnippetRequest represents the expected request body for creating a snippet.
type CreateSnippetRequest struct {
	Content   string   `json:"content" binding:"required"`
	ExpiresIn int      `json:"expires_in"`
	Tags      []string `json:"tags"`
}

// Create handles the creation of a new snippet.
func (h *Handler) Create(c *gin.Context) {
	var req CreateSnippetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(c, "failed to bind JSON: %s", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to bind JSON"})
		return
	}
	if len(req.Content) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}
	if len(req.Content) > 10*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content must be <= 10KB"})
		return
	}
	if req.ExpiresIn > 0 && req.ExpiresIn > 30*24*3600 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expires_in must be <= 30 days"})
		return
	}
	snippet, err := h.Svc.CreateSnippet(c, req.Content, req.ExpiresIn, req.Tags)
	if err != nil {
		logger.Error(c, "failed to create snippet: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"id":         snippet.ID,
		"created_at": snippet.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"expires_at": snippet.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z"),
		"tags":       snippet.Tags,
	})
}
