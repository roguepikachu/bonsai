// Package handler provides HTTP handlers for the API endpoints.
package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/roguepikachu/bonsai/internal/domain"
	"github.com/roguepikachu/bonsai/internal/service"
	"github.com/roguepikachu/bonsai/pkg/logger"
)

// Handler handles HTTP requests for snippets.
type Handler struct {
	Svc *service.Service
}

// Create handles the creation of a new snippet.
func (h *Handler) Create(c *gin.Context) {
	var req domain.CreateSnippetRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(c, "failed to bind JSON: %s", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "details": err.Error()})
		return
	}

	// Use validator for all rules
	if err := c.ShouldBind(&req); err != nil {
		logger.Error(c, "validation failed: %s", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation failed", "details": err.Error()})
		return
	}

	ctx := c.Request.Context()
	snippet, err := h.Svc.CreateSnippet(ctx, req.Content, req.ExpiresIn, req.Tags)
	if err != nil {
		logger.Error(c, "failed to create snippet: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	createdAt := snippet.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")
	var expiresAt *string
	if !snippet.ExpiresAt.IsZero() {
		v := snippet.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z")
		expiresAt = &v
	}
	resp := domain.SnippetResponseDTO{
		ID:        snippet.ID,
		Content:   snippet.Content,
		CreatedAt: createdAt,
		ExpiresAt: expiresAt,
		Tags:      snippet.Tags,
	}
	c.JSON(http.StatusCreated, resp)
}

// ...existing code...

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
	list := make([]domain.SnippetListItemDTO, 0, len(items))
	for _, s := range items {
		createdAt := s.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")
		var expiresAt *string
		if !s.ExpiresAt.IsZero() {
			v := s.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z")
			expiresAt = &v
		}
		list = append(list, domain.SnippetListItemDTO{
			ID:        s.ID,
			CreatedAt: createdAt,
			ExpiresAt: expiresAt,
		})
	}
	resp := domain.ListSnippetsResponseDTO{
		Page:  page,
		Limit: limit,
		Items: list,
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
	createdAt := snippet.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")
	var expiresAt *string
	if !snippet.ExpiresAt.IsZero() {
		v := snippet.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z")
		expiresAt = &v
	}
	resp := domain.SnippetResponseDTO{
		ID:        snippet.ID,
		Content:   snippet.Content,
		CreatedAt: createdAt,
		ExpiresAt: expiresAt,
		Tags:      snippet.Tags,
	}
	c.JSON(http.StatusOK, resp)
}
