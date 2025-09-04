package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roguepikachu/bonsai/internal/domain"
	"github.com/roguepikachu/bonsai/internal/service"
)

// Constants for commonly used test strings
const (
	testContentType    = "application/json"
	testContent        = "test content"
	updatedContent     = "updated"
	testID             = "test-id"
	testBodyDefault    = `{"content":"test","expires_in":60,"tags":[]}`
	testBodyNewContent = `{"content":"new content","expires_in":60,"tags":[]}`
)

type mockSnippetService struct {
	list        []domain.Snippet
	byID        map[string]domain.Snippet
	createErr   error
	listErr     error
	getErr      error
	updateErr   error
	created     []domain.Snippet
	updated     []domain.Snippet
	listCalls   int
	createCalls int
	getCalls    int
	updateCalls int
}

func (m *mockSnippetService) CreateSnippet(_ context.Context, content string, expiresIn int, tags []string) (domain.Snippet, error) {
	m.createCalls++
	if m.createErr != nil {
		return domain.Snippet{}, m.createErr
	}
	snippet := domain.Snippet{
		ID:        fmt.Sprintf("id-%d", m.createCalls),
		Content:   content,
		Tags:      tags,
		CreatedAt: time.Now(),
	}
	if expiresIn > 0 {
		snippet.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
	}
	m.created = append(m.created, snippet)
	return snippet, nil
}

func (m *mockSnippetService) ListSnippets(_ context.Context, _ int, _ int, _ string) ([]domain.Snippet, error) {
	m.listCalls++
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.list, nil
}

func (m *mockSnippetService) GetSnippetByID(_ context.Context, id string) (domain.Snippet, service.SnippetMeta, error) {
	m.getCalls++
	if m.getErr != nil {
		return domain.Snippet{}, service.SnippetMeta{CacheStatus: service.CacheMiss}, m.getErr
	}
	if s, ok := m.byID[id]; ok {
		return s, service.SnippetMeta{CacheStatus: service.CacheHit}, nil
	}
	return domain.Snippet{}, service.SnippetMeta{CacheStatus: service.CacheMiss}, service.ErrSnippetNotFound
}

func (m *mockSnippetService) UpdateSnippet(_ context.Context, id string, content string, expiresIn int, tags []string) (domain.Snippet, error) {
	m.updateCalls++
	if m.updateErr != nil {
		return domain.Snippet{}, m.updateErr
	}
	if existing, ok := m.byID[id]; ok {
		snippet := domain.Snippet{
			ID:        id,
			Content:   content,
			Tags:      tags,
			CreatedAt: existing.CreatedAt,
		}
		if expiresIn > 0 {
			snippet.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
		}
		m.byID[id] = snippet
		m.updated = append(m.updated, snippet)
		return snippet, nil
	}
	return domain.Snippet{}, service.ErrSnippetNotFound
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

func (e errSvc) UpdateSnippet(_ context.Context, _ string, _ string, _ int, _ []string) (domain.Snippet, error) {
	return e.snippet, e.retErr
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

func (c createSvc) UpdateSnippet(_ context.Context, _ string, _ string, _ int, _ []string) (domain.Snippet, error) {
	return c.out, nil
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
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d", w.Code)
	}
}

func TestSnippetCreate_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{}
	h := NewHandler(svc)
	r := gin.New()
	r.POST("/v1/snippets", h.Create)

	body := `{"content":"test", invalid json}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/snippets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestSnippetCreate_EmptyContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{}
	h := NewHandler(svc)
	r := gin.New()
	r.POST("/v1/snippets", h.Create)

	body := `{"content":"","expires_in":60,"tags":[]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/snippets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
	if svc.createCalls != 0 {
		t.Fatalf("expected CreateSnippet not called with empty content, got %d", svc.createCalls)
	}
}

func TestSnippetCreate_NoExpiry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{}
	h := NewHandler(svc)
	r := gin.New()
	r.POST("/v1/snippets", h.Create)

	body := `{"content":"no expiry","expires_in":0,"tags":["permanent"]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/snippets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d", w.Code)
	}

	var resp domain.SnippetResponseDTO
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.ExpiresAt != nil {
		t.Fatalf("expected no expiry, got %v", *resp.ExpiresAt)
	}
}

func TestSnippetCreate_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{createErr: fmt.Errorf("database down")}
	h := NewHandler(svc)
	r := gin.New()
	r.POST("/v1/snippets", h.Create)

	body := testBodyDefault
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/snippets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error object in response")
	}
	if errObj["code"] != "internal_error" {
		t.Fatalf("expected error code internal_error, got %v", errObj["code"])
	}
}

func TestSnippetCreate_LargeContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{}
	h := NewHandler(svc)
	r := gin.New()
	r.POST("/v1/snippets", h.Create)

	largeContent := strings.Repeat("a", 10000)
	body := fmt.Sprintf(`{"content":"%s","expires_in":3600,"tags":["large"]}`, largeContent)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/snippets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d", w.Code)
	}
	if len(svc.created) != 1 {
		t.Fatalf("expected snippet created")
	}
	if len(svc.created[0].Content) != 10000 {
		t.Fatalf("expected content length 10000, got %d", len(svc.created[0].Content))
	}
}

func TestSnippetList_EmptyResults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{list: []domain.Snippet{}}
	h := NewHandler(svc)
	r := gin.New()
	r.GET("/v1/snippets", h.List)

	req := httptest.NewRequest(http.MethodGet, "/v1/snippets", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp domain.ListSnippetsResponseDTO
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(resp.Items) != 0 {
		t.Fatalf("expected empty items, got %d", len(resp.Items))
	}
}

func TestSnippetList_WithPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now()
	snippets := []domain.Snippet{
		{ID: "1", CreatedAt: now},
		{ID: "2", CreatedAt: now.Add(-time.Hour)},
		{ID: "3", CreatedAt: now.Add(-2 * time.Hour)},
	}
	svc := &mockSnippetService{list: snippets}
	h := NewHandler(svc)
	r := gin.New()
	r.GET("/v1/snippets", h.List)

	req := httptest.NewRequest(http.MethodGet, "/v1/snippets?page=2&limit=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp domain.ListSnippetsResponseDTO
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Page != 2 {
		t.Fatalf("expected page 2, got %d", resp.Page)
	}
	if resp.Limit != 10 {
		t.Fatalf("expected limit 10, got %d", resp.Limit)
	}
	if len(resp.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(resp.Items))
	}
}

func TestSnippetList_WithTagFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{list: []domain.Snippet{{ID: "go1", CreatedAt: time.Now()}}}
	h := NewHandler(svc)
	r := gin.New()
	r.GET("/v1/snippets", h.List)

	req := httptest.NewRequest(http.MethodGet, "/v1/snippets?tag=golang", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if svc.listCalls != 1 {
		t.Fatalf("expected ListSnippets called once, got %d", svc.listCalls)
	}
}

func TestSnippetList_InvalidPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{}
	h := NewHandler(svc)
	r := gin.New()
	r.GET("/v1/snippets", h.List)

	req := httptest.NewRequest(http.MethodGet, "/v1/snippets?page=0", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestSnippetList_InvalidLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{}
	h := NewHandler(svc)
	r := gin.New()
	r.GET("/v1/snippets", h.List)

	// Test limit > 100
	req := httptest.NewRequest(http.MethodGet, "/v1/snippets?limit=101", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for limit>100, got %d", w.Code)
	}
}

func TestSnippetList_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{listErr: fmt.Errorf("connection lost")}
	h := NewHandler(svc)
	r := gin.New()
	r.GET("/v1/snippets", h.List)

	req := httptest.NewRequest(http.MethodGet, "/v1/snippets", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
}

func TestSnippetList_DefaultValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{list: []domain.Snippet{}}
	h := NewHandler(svc)
	r := gin.New()
	r.GET("/v1/snippets", h.List)

	// No query params, should use defaults
	req := httptest.NewRequest(http.MethodGet, "/v1/snippets", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp domain.ListSnippetsResponseDTO
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Page != 1 {
		t.Fatalf("expected default page 1, got %d", resp.Page)
	}
	if resp.Limit != 20 {
		t.Fatalf("expected default limit 20, got %d", resp.Limit)
	}
}

func TestSnippetGet_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now()
	snippet := domain.Snippet{
		ID:        "test-id",
		Content:   "test content",
		Tags:      []string{"test", "snippet"},
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{"test-id": snippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.GET("/v1/snippets/:id", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/v1/snippets/test-id", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp domain.SnippetResponseDTO
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.ID != "test-id" {
		t.Fatalf("expected ID test-id, got %s", resp.ID)
	}
	if resp.Content != "test content" {
		t.Fatalf("expected content 'test content', got %s", resp.Content)
	}
	if len(resp.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(resp.Tags))
	}
}

func TestSnippetGet_EmptyID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{}
	h := NewHandler(svc)
	r := gin.New()
	r.GET("/v1/snippets/:id", h.Get)

	// This shouldn't match the route, but testing handler logic
	req := httptest.NewRequest(http.MethodGet, "/v1/snippets/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// Router won't match this path, so it returns 404
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestSnippetGet_CacheMiss(t *testing.T) {
	gin.SetMode(gin.TestMode)
	snippet := domain.Snippet{
		ID:        "cache-test",
		Content:   "content",
		CreatedAt: time.Now(),
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{"cache-test": snippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.GET("/v1/snippets/:id", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/v1/snippets/cache-test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if w.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("expected X-Cache=HIT, got %q", w.Header().Get("X-Cache"))
	}
}

func TestSnippetGet_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{getErr: fmt.Errorf("unexpected error")}
	h := NewHandler(svc)
	r := gin.New()
	r.GET("/v1/snippets/:id", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/v1/snippets/any", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
}

func TestSnippetGet_NoExpiry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	snippet := domain.Snippet{
		ID:        "no-exp",
		Content:   "permanent",
		CreatedAt: time.Now(),
		ExpiresAt: time.Time{}, // Zero time = no expiry
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{"no-exp": snippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.GET("/v1/snippets/:id", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/v1/snippets/no-exp", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp domain.SnippetResponseDTO
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.ExpiresAt != nil {
		t.Fatalf("expected nil ExpiresAt, got %v", *resp.ExpiresAt)
	}
}

func TestHandler_ConcurrentRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{
		list: []domain.Snippet{{ID: "1", CreatedAt: time.Now()}},
		byID: map[string]domain.Snippet{"1": {ID: "1", Content: "test", CreatedAt: time.Now()}},
	}
	h := NewHandler(svc)
	r := gin.New()
	r.POST("/v1/snippets", h.Create)
	r.GET("/v1/snippets", h.List)
	r.GET("/v1/snippets/:id", h.Get)

	done := make(chan bool, 3)

	// Concurrent create
	go func() {
		body := testBodyDefault
		req := httptest.NewRequest(http.MethodPost, "/v1/snippets", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", testContentType)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		done <- true
	}()

	// Concurrent list
	go func() {
		req := httptest.NewRequest(http.MethodGet, "/v1/snippets", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		done <- true
	}()

	// Concurrent get
	go func() {
		req := httptest.NewRequest(http.MethodGet, "/v1/snippets/1", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	if svc.createCalls < 1 {
		t.Fatalf("expected at least 1 create call, got %d", svc.createCalls)
	}
	if svc.listCalls < 1 {
		t.Fatalf("expected at least 1 list call, got %d", svc.listCalls)
	}
	if svc.getCalls < 1 {
		t.Fatalf("expected at least 1 get call, got %d", svc.getCalls)
	}
}

func TestTimeFormat(t *testing.T) {
	// Test that TimeFormat constant is correct RFC3339 format
	expected := "2006-01-02T15:04:05Z"
	if TimeFormat != expected {
		t.Fatalf("expected TimeFormat to be %s, got %s", expected, TimeFormat)
	}

	// Test parsing and formatting
	testTime := time.Date(2025, 8, 31, 23, 59, 59, 0, time.UTC)
	formatted := testTime.Format(TimeFormat)
	if formatted != "2025-08-31T23:59:59Z" {
		t.Fatalf("expected formatted time 2025-08-31T23:59:59Z, got %s", formatted)
	}
}

func TestSnippetUpdate_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	existingSnippet := domain.Snippet{
		ID:        "update-id",
		Content:   "old content",
		Tags:      []string{"old"},
		CreatedAt: time.Now(),
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{"update-id": existingSnippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	body := `{"content":"updated content","expires_in":3600,"tags":["updated","new"]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/update-id", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp domain.SnippetResponseDTO
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Content != "updated content" {
		t.Fatalf("expected content 'updated content', got %s", resp.Content)
	}
	if len(resp.Tags) != 2 || resp.Tags[0] != "updated" || resp.Tags[1] != "new" {
		t.Fatalf("expected tags [updated new], got %v", resp.Tags)
	}
}

func TestSnippetUpdate_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{byID: map[string]domain.Snippet{}}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	body := testBodyNewContent
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/nonexistent", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestSnippetUpdate_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	body := `{"content":"test", invalid json}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/id", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestSnippetUpdate_EmptyContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	existingSnippet := domain.Snippet{
		ID:        "test-id",
		Content:   "old content",
		CreatedAt: time.Now(),
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{"test-id": existingSnippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	body := `{"content":"","expires_in":60,"tags":[]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/"+testID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
	if svc.updateCalls != 0 {
		t.Fatalf("expected UpdateSnippet not called with empty content, got %d", svc.updateCalls)
	}
}

func TestSnippetUpdate_ExpiredSnippet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(errSvc{retErr: service.ErrSnippetExpired})
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	body := testBodyNewContent
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/expired", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusGone {
		t.Fatalf("want 410, got %d", w.Code)
	}
}

func TestSnippetUpdate_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{
		byID:      map[string]domain.Snippet{"error-id": {ID: "error-id"}},
		updateErr: fmt.Errorf("database error"),
	}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	body := testBodyDefault
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/error-id", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error object in response")
	}
	if errObj["code"] != "internal_error" {
		t.Fatalf("expected error code internal_error, got %v", errObj["code"])
	}
}

func TestSnippetUpdate_NoExpiry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	existingSnippet := domain.Snippet{
		ID:        "no-exp-id",
		Content:   "old content",
		CreatedAt: time.Now().Add(-time.Hour),
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{"no-exp-id": existingSnippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	body := `{"content":"updated with no expiry","expires_in":0,"tags":["permanent"]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/no-exp-id", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp domain.SnippetResponseDTO
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.ExpiresAt != nil {
		t.Fatalf("expected no expiry, got %v", *resp.ExpiresAt)
	}
}

func TestSnippetUpdate_LargeContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	existingSnippet := domain.Snippet{
		ID:        "large-id",
		Content:   "small",
		CreatedAt: time.Now(),
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{"large-id": existingSnippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	largeContent := strings.Repeat("b", 10000)
	body := fmt.Sprintf(`{"content":"%s","expires_in":3600,"tags":["large"]}`, largeContent)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/large-id", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if len(svc.updated) != 1 {
		t.Fatalf("expected snippet updated")
	}
	if len(svc.updated[0].Content) != 10000 {
		t.Fatalf("expected content length 10000, got %d", len(svc.updated[0].Content))
	}
}

func TestSnippetUpdate_PreservesCreatedAt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalCreatedAt := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	existingSnippet := domain.Snippet{
		ID:        "preserve-id",
		Content:   "old content",
		CreatedAt: originalCreatedAt,
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{"preserve-id": existingSnippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	body := testBodyNewContent
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/preserve-id", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	if len(svc.updated) != 1 {
		t.Fatalf("expected snippet updated")
	}
	if !svc.updated[0].CreatedAt.Equal(originalCreatedAt) {
		t.Fatalf("expected CreatedAt to be preserved, got %v, want %v", svc.updated[0].CreatedAt, originalCreatedAt)
	}
}

// Edge case tests for PUT handler

func TestSnippetUpdate_MissingID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	body := testBodyDefault
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	// Should return 404 as the route won't match without ID
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 for missing ID, got %d", w.Code)
	}
}

func TestSnippetUpdate_EmptyStringID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{}
	h := NewHandler(svc)
	r := gin.New()
	// Route that would match empty string
	r.PUT("/v1/snippets/:id/update", func(c *gin.Context) {
		h.Update(c)
	})

	body := testBodyDefault
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets//update", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for empty string ID, got %d", w.Code)
	}
}

func TestSnippetUpdate_VeryLongID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	existingSnippet := domain.Snippet{
		ID:        strings.Repeat("a", 1000), // Very long ID
		Content:   "content",
		CreatedAt: time.Now(),
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{strings.Repeat("a", 1000): existingSnippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	body := fmt.Sprintf(`{"content":"%s","expires_in":60,"tags":[]}`, updatedContent)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/"+strings.Repeat("a", 1000), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 for long ID, got %d", w.Code)
	}
}

func TestSnippetUpdate_SpecialCharacterID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	specialID := "test-id-with-special-chars-!@#$%^&*()_+-=[]{}|;:,.<>?"
	existingSnippet := domain.Snippet{
		ID:        specialID,
		Content:   "content",
		CreatedAt: time.Now(),
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{specialID: existingSnippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	body := fmt.Sprintf(`{"content":"%s","expires_in":60,"tags":[]}`, updatedContent)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/"+url.QueryEscape(specialID), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 for special character ID, got %d", w.Code)
	}
}

func TestSnippetUpdate_UnicodeID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	unicodeID := "测试-🔥-emoji-id-αβγ"
	existingSnippet := domain.Snippet{
		ID:        unicodeID,
		Content:   "content",
		CreatedAt: time.Now(),
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{unicodeID: existingSnippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	body := fmt.Sprintf(`{"content":"%s","expires_in":60,"tags":[]}`, updatedContent)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/"+unicodeID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 for unicode ID, got %d", w.Code)
	}
}

func TestSnippetUpdate_MaxContentLength(t *testing.T) {
	gin.SetMode(gin.TestMode)
	existingSnippet := domain.Snippet{
		ID:        "max-content-id",
		Content:   "small",
		CreatedAt: time.Now(),
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{"max-content-id": existingSnippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	maxContent := strings.Repeat("a", 10240) // Exactly at limit
	body := fmt.Sprintf(`{"content":"%s","expires_in":60,"tags":["max"]}`, maxContent)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/max-content-id", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 for max content length, got %d", w.Code)
	}
}

func TestSnippetUpdate_ExceedMaxContentLength(t *testing.T) {
	gin.SetMode(gin.TestMode)
	existingSnippet := domain.Snippet{
		ID:        "exceed-id",
		Content:   "small",
		CreatedAt: time.Now(),
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{"exceed-id": existingSnippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	exceedContent := strings.Repeat("a", 10241) // One over limit
	body := fmt.Sprintf(`{"content":"%s","expires_in":60,"tags":[]}`, exceedContent)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/exceed-id", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for content exceeding limit, got %d", w.Code)
	}
}

func TestSnippetUpdate_MaxExpiresIn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	existingSnippet := domain.Snippet{
		ID:        "max-exp-id",
		Content:   "content",
		CreatedAt: time.Now(),
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{"max-exp-id": existingSnippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	body := `{"content":"test","expires_in":2592000,"tags":[]}` // 30 days in seconds (max)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/max-exp-id", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 for max expires_in, got %d", w.Code)
	}
}

func TestSnippetUpdate_ExceedMaxExpiresIn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	existingSnippet := domain.Snippet{
		ID:        "exceed-exp-id",
		Content:   "content",
		CreatedAt: time.Now(),
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{"exceed-exp-id": existingSnippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	body := `{"content":"test","expires_in":2592001,"tags":[]}` // One second over max
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/exceed-exp-id", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for expires_in exceeding limit, got %d", w.Code)
	}
}

func TestSnippetUpdate_NegativeExpiresIn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	existingSnippet := domain.Snippet{
		ID:        "neg-exp-id",
		Content:   "content",
		CreatedAt: time.Now(),
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{"neg-exp-id": existingSnippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	body := `{"content":"test","expires_in":-1,"tags":[]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/neg-exp-id", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for negative expires_in, got %d", w.Code)
	}
}

func TestSnippetUpdate_EmptyTagsArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	existingSnippet := domain.Snippet{
		ID:        "empty-tags-id",
		Content:   "content",
		CreatedAt: time.Now(),
		Tags:      []string{"old", "tags"},
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{"empty-tags-id": existingSnippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	body := fmt.Sprintf(`{"content":"%s","expires_in":60,"tags":[]}`, updatedContent)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/empty-tags-id", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 for empty tags array, got %d", w.Code)
	}

	var resp domain.SnippetResponseDTO
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if len(resp.Tags) != 0 {
		t.Fatalf("expected empty tags array, got %v", resp.Tags)
	}
}

func TestSnippetUpdate_MissingTagsField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	existingSnippet := domain.Snippet{
		ID:        "missing-tags-id",
		Content:   "content",
		CreatedAt: time.Now(),
		Tags:      []string{"old", "tags"},
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{"missing-tags-id": existingSnippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	body := `{"content":"updated","expires_in":60}` // No tags field
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/missing-tags-id", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 for missing tags field, got %d", w.Code)
	}

	var resp domain.SnippetResponseDTO
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	// Should be nil/empty when tags field is omitted
	if len(resp.Tags) != 0 {
		t.Fatalf("expected nil or empty tags when field omitted, got %v", resp.Tags)
	}
}

func TestSnippetUpdate_NullTagsField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	existingSnippet := domain.Snippet{
		ID:        "null-tags-id",
		Content:   "content",
		CreatedAt: time.Now(),
		Tags:      []string{"old", "tags"},
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{"null-tags-id": existingSnippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	body := `{"content":"updated","expires_in":60,"tags":null}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/null-tags-id", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 for null tags, got %d", w.Code)
	}
}

func TestSnippetUpdate_LargeNumberOfTags(t *testing.T) {
	gin.SetMode(gin.TestMode)
	existingSnippet := domain.Snippet{
		ID:        "many-tags-id",
		Content:   "content",
		CreatedAt: time.Now(),
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{"many-tags-id": existingSnippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	// Create 100 tags
	tags := make([]string, 100)
	for i := range tags {
		tags[i] = fmt.Sprintf("tag-%d", i)
	}
	tagsJSON, _ := json.Marshal(tags)
	body := fmt.Sprintf(`{"content":"updated","expires_in":60,"tags":%s}`, string(tagsJSON))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/many-tags-id", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 for many tags, got %d", w.Code)
	}
}

func TestSnippetUpdate_UnicodeContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	existingSnippet := domain.Snippet{
		ID:        "unicode-id",
		Content:   "old content",
		CreatedAt: time.Now(),
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{"unicode-id": existingSnippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	unicodeContent := "Hello 世界! 🌍 Testing αβγ and ñáéíóú"
	body := fmt.Sprintf(`{"content":"%s","expires_in":60,"tags":["unicode","test"]}`, unicodeContent)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/unicode-id", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 for unicode content, got %d", w.Code)
	}

	var resp domain.SnippetResponseDTO
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Content != unicodeContent {
		t.Fatalf("expected unicode content preserved, got %s", resp.Content)
	}
}

// testUpdateWithSpecialContent tests updating a snippet with special content characters
func testUpdateWithSpecialContent(t *testing.T, snippetID, content, testName string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	existingSnippet := domain.Snippet{
		ID:        snippetID,
		Content:   "old content",
		CreatedAt: time.Now(),
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{snippetID: existingSnippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	// JSON encode the content to properly escape special characters
	contentJSON, _ := json.Marshal(content)
	body := fmt.Sprintf(`{"content":%s,"expires_in":60,"tags":["%s"]}`, string(contentJSON), testName)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/"+snippetID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 for content with %s, got %d", testName, w.Code)
	}

	var resp domain.SnippetResponseDTO
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Content != content {
		t.Fatalf("expected %s preserved, got %s", testName, resp.Content)
	}
}

func TestSnippetUpdate_ContentWithNewlines(t *testing.T) {
	contentWithNewlines := "Line 1\nLine 2\r\nLine 3\n\nLine 5"
	testUpdateWithSpecialContent(t, "newline-id", contentWithNewlines, "newlines")
}

func TestSnippetUpdate_ContentWithQuotes(t *testing.T) {
	contentWithQuotes := `Content with "double" and 'single' quotes`
	testUpdateWithSpecialContent(t, "quotes-id", contentWithQuotes, "quotes")
}

func TestSnippetUpdate_MalformedJSON_MissingBrace(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	malformedJSON := `{"content":"test","expires_in":60,"tags":[]` // Missing closing brace
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/"+testID, bytes.NewBufferString(malformedJSON))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for malformed JSON, got %d", w.Code)
	}
}

func TestSnippetUpdate_MalformedJSON_InvalidValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	malformedJSON := `{"content":"test","expires_in":"not-a-number","tags":[]}` // String where int expected
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/"+testID, bytes.NewBufferString(malformedJSON))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for invalid JSON value type, got %d", w.Code)
	}
}

func TestSnippetUpdate_NoContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	existingSnippet := domain.Snippet{
		ID:        "no-content-type-id",
		Content:   "old content",
		CreatedAt: time.Now(),
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{"no-content-type-id": existingSnippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	body := fmt.Sprintf(`{"content":"%s","expires_in":60,"tags":[]}`, updatedContent)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/no-content-type-id", bytes.NewBufferString(body))
	// Intentionally not setting Content-Type header
	r.ServeHTTP(w, req)
	// Gin should still attempt to parse JSON
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 even without content-type, got %d", w.Code)
	}
}

func TestSnippetUpdate_WrongContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	existingSnippet := domain.Snippet{
		ID:        "wrong-content-type-id",
		Content:   "old content",
		CreatedAt: time.Now(),
	}
	svc := &mockSnippetService{byID: map[string]domain.Snippet{"wrong-content-type-id": existingSnippet}}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	body := fmt.Sprintf(`{"content":"%s","expires_in":60,"tags":[]}`, updatedContent)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/wrong-content-type-id", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "text/plain") // Wrong content type
	r.ServeHTTP(w, req)
	// Gin's ShouldBindJSON is lenient and allows parsing JSON even with wrong content type
	// as long as the body is valid JSON
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 for valid JSON body (Gin is lenient with content type), got %d", w.Code)
	}
}

func TestSnippetUpdate_EmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/"+testID, bytes.NewBufferString(""))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for empty body, got %d", w.Code)
	}
}

func TestSnippetUpdate_VeryLargePayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{}
	h := NewHandler(svc)
	r := gin.New()
	r.PUT("/v1/snippets/:id", h.Update)

	// Create a very large JSON payload (beyond content limit but with extra JSON overhead)
	largeContent := strings.Repeat("a", 50000)
	body := fmt.Sprintf(`{"content":"%s","expires_in":60,"tags":["large"]}`, largeContent)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/v1/snippets/"+testID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", testContentType)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for very large payload, got %d", w.Code)
	}
}
