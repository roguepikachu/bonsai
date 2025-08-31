package handler

import (
	"bytes"
	"context"
	"errors"
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

// errSvc implements SnippetService and allows controlling GetSnippetByID results.
type errSvc struct {
	retErr  error
	snippet domain.Snippet
	meta    service.SnippetMeta
}

func (errSvc) CreateSnippet(_ context.Context, _ string, _ int, _ []string) (domain.Snippet, error) {
	return domain.Snippet{}, nil
}

func (errSvc) ListSnippets(_ context.Context, _ int, _ int, _ string) ([]domain.Snippet, error) {
	return nil, nil
}

func (e errSvc) GetSnippetByID(_ context.Context, _ string) (domain.Snippet, service.SnippetMeta, error) {
	return e.snippet, e.meta, e.retErr
}

// createSvc returns a fixed snippet for CreateSnippet to test the happy path.
type createSvc struct{ out domain.Snippet }

func (c createSvc) CreateSnippet(_ context.Context, _ string, _ int, _ []string) (domain.Snippet, error) {
	return c.out, nil
}

func (createSvc) ListSnippets(_ context.Context, _ int, _ int, _ string) ([]domain.Snippet, error) {
	return nil, nil
}

func (createSvc) GetSnippetByID(_ context.Context, _ string) (domain.Snippet, service.SnippetMeta, error) {
	return domain.Snippet{}, service.SnippetMeta{}, nil
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

func TestSnippetList_BadParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{}
	h := NewHandler(svc)
	r := gin.New()
	r.GET("/v1/snippets", h.List)

	// limit=0 should fail binding (gte=1)
	req := httptest.NewRequest(http.MethodGet, "/v1/snippets?limit=0", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestSnippetGet_ExpiredAndInternal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(errSvc{})
	r := gin.New()
	r.GET("/v1/snippets/:id", h.Get)

	// Expired
	h = NewHandler(errSvc{retErr: service.ErrSnippetExpired, meta: service.SnippetMeta{CacheStatus: service.CacheMiss}})
	r = gin.New()
	r.GET("/v1/snippets/:id", h.Get)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/snippets/old", nil))
	if w.Code != http.StatusGone {
		t.Fatalf("want 410, got %d", w.Code)
	}

	// Internal error
	h = NewHandler(errSvc{retErr: errors.New("boom"), meta: service.SnippetMeta{CacheStatus: service.CacheMiss}})
	r = gin.New()
	r.GET("/v1/snippets/:id", h.Get)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/snippets/err", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
}

func TestSnippetGet_XCacheHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(errSvc{snippet: domain.Snippet{ID: "a", CreatedAt: time.Now()}, meta: service.SnippetMeta{CacheStatus: service.CacheHit}})
	r := gin.New()
	r.GET("/v1/snippets/:id", h.Get)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/snippets/a", nil))
	if w.Header().Get("X-Cache") != string(service.CacheHit) {
		t.Fatalf("want X-Cache=HIT, got %q", w.Header().Get("X-Cache"))
	}
}

func TestSnippetCreate_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	created := time.Date(2025, 8, 31, 16, 0, 0, 0, time.UTC)
	expires := created.Add(90 * time.Second)
	h := NewHandler(createSvc{out: domain.Snippet{ID: "c1", Content: "hi", CreatedAt: created, ExpiresAt: expires, Tags: []string{"t1", "t2"}}})
	r := gin.New()
	r.POST("/v1/snippets", h.Create)

	body := `{"content":"hi","expires_in":90,"tags":["t1","t2"]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/snippets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d", w.Code)
	}
}
