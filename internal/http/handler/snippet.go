package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/roguepikachu/bonsai/internal/domain"
	"github.com/roguepikachu/bonsai/internal/service"
	"github.com/roguepikachu/bonsai/pkg/logger"
)

const (
	// TimeFormat is the standard format for time serialization.
	TimeFormat = "2006-01-02T15:04:05Z"
)

// SnippetService defines the handler's dependency contract.
type SnippetService interface {
	CreateSnippet(ctx context.Context, content string, expiresIn int, tags []string) (domain.Snippet, error)
	ListSnippets(ctx context.Context, page, limit int, tag string) ([]domain.Snippet, error)
	GetSnippetByID(ctx context.Context, id string) (domain.Snippet, service.SnippetMeta, error)
}

// Handler handles HTTP requests for snippets.
type Handler struct {
	svc SnippetService
}

// NewHandler constructs a Handler with the given SnippetService.
func NewHandler(svc SnippetService) *Handler {
	return &Handler{svc: svc}
}

// Create handles the creation of a new snippet.
func (h *Handler) Create(c *gin.Context) {
	ctx := c.Request.Context()
	var req domain.CreateSnippetRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "failed to bind JSON: %s", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "invalid request", "details": err.Error()}})
		return
	}

	snippet, err := h.svc.CreateSnippet(ctx, req.Content, req.ExpiresIn, req.Tags)
	if err != nil {
		logger.Error(ctx, "failed to create snippet: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal_error", "message": "internal server error"}})
		return
	}
	logger.With(ctx, map[string]any{"id": snippet.ID, "tags": snippet.Tags}).Info("snippet created")
	createdAt := snippet.CreatedAt.UTC().Format(TimeFormat)
	var expiresAt *string
	if !snippet.ExpiresAt.IsZero() {
		v := snippet.ExpiresAt.UTC().Format(TimeFormat)
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

// List handles listing all snippets with pagination and optional tag filter.
func (h *Handler) List(c *gin.Context) {
	ctx := c.Request.Context()
	type queryParams struct {
		Page  int    `form:"page,default=1" binding:"gte=1"`
		Limit int    `form:"limit,default=20" binding:"gte=1,lte=100"`
		Tag   string `form:"tag"`
	}
	var q queryParams
	if err := c.ShouldBindQuery(&q); err != nil {
		logger.Error(ctx, "invalid query params: %s", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "invalid query parameters", "details": err.Error()}})
		return
	}
	// Cap pagination defensively
	if q.Limit < 1 {
		q.Limit = service.ServiceDefaultLimit
	}
	if q.Limit > service.ServiceMaxLimit {
		q.Limit = service.ServiceMaxLimit
	}
	if q.Page < 1 {
		q.Page = service.ServiceDefaultPage
	}
	items, err := h.svc.ListSnippets(ctx, q.Page, q.Limit, q.Tag)
	if err != nil {
		logger.Error(ctx, "failed to list snippets: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal_error", "message": "internal server error"}})
		return
	}
	logger.With(ctx, map[string]any{"count": len(items), "page": q.Page, "limit": q.Limit, "tag": q.Tag}).Debug("snippets listed")
	list := make([]domain.SnippetListItemDTO, 0, len(items))
	for _, s := range items {
		createdAt := s.CreatedAt.UTC().Format(TimeFormat)
		var expiresAt *string
		if !s.ExpiresAt.IsZero() {
			v := s.ExpiresAt.UTC().Format(TimeFormat)
			expiresAt = &v
		}
		list = append(list, domain.SnippetListItemDTO{
			ID:        s.ID,
			CreatedAt: createdAt,
			ExpiresAt: expiresAt,
		})
	}
	resp := domain.ListSnippetsResponseDTO{
		Page:  q.Page,
		Limit: q.Limit,
		Items: list,
	}
	c.JSON(http.StatusOK, resp)
}

// Get handles fetching a snippet by ID.
func (h *Handler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "bad_request", "message": "id is required"}})
		return
	}
	snippet, meta, err := h.svc.GetSnippetByID(ctx, id)
	cacheStatus := string(meta.CacheStatus)
	if err != nil {
		if errors.Is(err, service.ErrSnippetNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "not found"}})
			return
		}
		if errors.Is(err, service.ErrSnippetExpired) {
			c.JSON(http.StatusGone, gin.H{"error": gin.H{"code": "gone", "message": "expired"}})
			return
		}
		logger.Error(ctx, "failed to get snippet: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal_error", "message": "internal server error"}})
		return
	}
	logger.With(ctx, map[string]any{"id": id, "cache": cacheStatus}).Debug("snippet retrieved")
	c.Header("X-Cache", cacheStatus)
	createdAt := snippet.CreatedAt.UTC().Format(TimeFormat)
	var expiresAt *string
	if !snippet.ExpiresAt.IsZero() {
		v := snippet.ExpiresAt.UTC().Format(TimeFormat)
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
