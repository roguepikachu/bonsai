package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roguepikachu/bonsai/internal/domain"
	h "github.com/roguepikachu/bonsai/internal/http/handler"
	"github.com/roguepikachu/bonsai/internal/service"
)

// test service implementing handler.SnippetService
type testSvc struct{}

func (testSvc) CreateSnippet(_ context.Context, _ string, _ int, _ []string) (domain.Snippet, error) {
	return domain.Snippet{ID: "x", CreatedAt: time.Now()}, nil
}
func (testSvc) ListSnippets(_ context.Context, _ int, _ int, _ string) ([]domain.Snippet, error) {
	return []domain.Snippet{}, nil
}
func (testSvc) GetSnippetByID(_ context.Context, _ string) (domain.Snippet, service.SnippetMeta, error) {
	return domain.Snippet{}, service.SnippetMeta{CacheStatus: service.CacheMiss}, service.ErrSnippetNotFound
}

func TestNewRouter_RoutesBasic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(h.NewHandler(testSvc{}), h.NewHealthHandler(nil, nil))

	// Health
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/v1/health want 200, got %d", w.Code)
	}

	// Liveness
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/livez", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/v1/livez want 200, got %d", w.Code)
	}

	// Readiness (no deps -> ready)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/readyz", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/v1/readyz want 200, got %d", w.Code)
	}

	// List snippets
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/snippets", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /v1/snippets want 200, got %d", w.Code)
	}

	// Create snippet with empty body -> 400 due to validation
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/snippets", nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("POST /v1/snippets want 400, got %d", w.Code)
	}

	// Get by id (not found)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/snippets/nope", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("GET /v1/snippets/:id want 404, got %d", w.Code)
	}
}
