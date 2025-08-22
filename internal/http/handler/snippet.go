// Package handler provides HTTP handlers for the API endpoints.
package handler

import (
	"net/http"
	"strconv"

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
	ctx := c.Request.Context()
	snippet, err := h.Svc.CreateSnippet(ctx, req.Content, req.ExpiresIn, req.Tags)
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

// ListSnippetsResponse represents the response for listing snippets.
type ListSnippetsResponse struct {
	Page  int              `json:"page"`
	Limit int              `json:"limit"`
	Items []map[string]any `json:"items"`
}

// List handles listing all snippets with pagination and optional tag filter.
func (h *Handler) List(c *gin.Context) {
	page := 1
	limit := 20
	tag := c.Query("tag")
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	ctx := c.Request.Context()
	items, err := h.Svc.ListSnippets(ctx, page, limit, tag)
	if err != nil {
		logger.Error(c, "failed to list snippets: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	resp := ListSnippetsResponse{
		Page:  page,
		Limit: limit,
		Items: make([]map[string]any, 0, len(items)),
	}
	for _, s := range items {
		item := map[string]any{
			"id":         s.ID,
			"created_at": s.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		}
		if !s.ExpiresAt.IsZero() {
			item["expires_at"] = s.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z")
		} else {
			item["expires_at"] = nil
		}
		resp.Items = append(resp.Items, item)
	}
	c.JSON(http.StatusOK, resp)
}

// Get handles fetching a snippet by ID.
func (h *Handler) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	ctx := c.Request.Context()
	snippet, cacheStatus, err := h.Svc.GetSnippetByID(ctx, id)
	if err != nil {
		if err == service.ErrSnippetNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err == service.ErrSnippetExpired {
			c.JSON(http.StatusGone, gin.H{"error": "expired"})
			return
		}
		logger.Error(c, "failed to get snippet: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.Header("X-Cache", cacheStatus)
	c.JSON(http.StatusOK, gin.H{
		"id":         snippet.ID,
		"content":    snippet.Content,
		"created_at": snippet.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		"expires_at": func() any {
			if snippet.ExpiresAt.IsZero() {
				return nil
			}
			return snippet.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z")
		}(),
	})
}
