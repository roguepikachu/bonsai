package handler

// Package handler provides HTTP handlers for the API endpoints.

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
	DefaultPage     = 1
	DefaultLimit    = 20
	MaxLimit        = 100
	MaxSnippetBytes = 10 * 1024
	MaxExpirySecs   = 30 * 24 * 3600
	TimeFormat      = "2006-01-02T15:04:05Z"
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
	var req domain.CreateSnippetRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(c, "failed to bind JSON: %s", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "details": err.Error()})
		return
	}

	ctx := c.Request.Context()
	snippet, err := h.svc.CreateSnippet(ctx, req.Content, req.ExpiresIn, req.Tags)
	if err != nil {
		logger.Error(c, "failed to create snippet: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
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
	type queryParams struct {
		Page  int    `form:"page,default=1" binding:"gte=1"`
		Limit int    `form:"limit,default=20" binding:"gte=1,lte=100"`
		Tag   string `form:"tag"`
	}
	var q queryParams
	if err := c.ShouldBindQuery(&q); err != nil {
		logger.Error(c, "invalid query params: %s", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query parameters", "details": err.Error()})
		return
	}
	// Cap pagination
	if q.Limit > MaxLimit {
		q.Limit = MaxLimit
	}
	if q.Limit < 1 {
		q.Limit = DefaultLimit
	}
	if q.Page < 1 {
		q.Page = DefaultPage
	}
	ctx := c.Request.Context()
	items, err := h.svc.ListSnippets(ctx, q.Page, q.Limit, q.Tag)
	if err != nil {
		logger.Error(c, "failed to list snippets: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
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
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	ctx := c.Request.Context()
	snippet, meta, err := h.svc.GetSnippetByID(ctx, id)
	cacheStatus := string(meta.CacheStatus)
	if err != nil {
		if errors.Is(err, service.ErrSnippetNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if errors.Is(err, service.ErrSnippetExpired) {
			c.JSON(http.StatusGone, gin.H{"error": "expired"})
			return
		}
		logger.Error(c, "failed to get snippet: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
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
