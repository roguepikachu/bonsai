package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roguepikachu/bonsai/internal/domain"
	"github.com/roguepikachu/bonsai/internal/service"
)

type mockSnippetService struct {
	list []domain.Snippet
	byID map[string]domain.Snippet
}

func (m *mockSnippetService) CreateSnippet(_ context.Context, _ string, _ int, _ []string) (domain.Snippet, error) {
	return domain.Snippet{}, nil
}
func (m *mockSnippetService) ListSnippets(_ context.Context, _ int, _ int, _ string) ([]domain.Snippet, error) {
	return m.list, nil
}
func (m *mockSnippetService) GetSnippetByID(_ context.Context, id string) (domain.Snippet, service.SnippetMeta, error) {
	if s, ok := m.byID[id]; ok {
		return s, service.SnippetMeta{CacheStatus: service.CacheHit}, nil
	}
	return domain.Snippet{}, service.SnippetMeta{CacheStatus: service.CacheMiss}, service.ErrSnippetNotFound
}

func TestSnippetList_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{list: []domain.Snippet{{ID: "a", CreatedAt: time.Now()}}}
	h := NewHandler(svc)
	r := gin.New()
	r.GET("/v1/snippets", h.List)

	req := httptest.NewRequest(http.MethodGet, "/v1/snippets?page=1&limit=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

func TestSnippetGet_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{byID: map[string]domain.Snippet{}}
	h := NewHandler(svc)
	r := gin.New()
	r.GET("/v1/snippets/:id", h.Get)
	req := httptest.NewRequest(http.MethodGet, "/v1/snippets/nope", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}
