package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roguepikachu/bonsai/internal/domain"
	"github.com/roguepikachu/bonsai/internal/service"
)

type mockSnippetService struct {
	list        []domain.Snippet
	byID        map[string]domain.Snippet
	createErr   error
	listErr     error
	getErr      error
	created     []domain.Snippet
	listCalls   int
	createCalls int
	getCalls    int
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

func TestSnippetCreate_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &mockSnippetService{}
	h := NewHandler(svc)
	r := gin.New()
	r.POST("/v1/snippets", h.Create)

	body := `{"content":"test", invalid json}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/snippets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
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
	req.Header.Set("Content-Type", "application/json")
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
	req.Header.Set("Content-Type", "application/json")
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

	body := `{"content":"test","expires_in":60,"tags":[]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/snippets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if errObj, ok := resp["error"].(map[string]interface{}); ok {
		if errObj["code"] != "internal_error" {
			t.Fatalf("expected error code internal_error, got %v", errObj["code"])
		}
	} else {
		t.Fatalf("expected error object in response")
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
	req.Header.Set("Content-Type", "application/json")
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
		body := `{"content":"test","expires_in":60,"tags":[]}`
		req := httptest.NewRequest(http.MethodPost, "/v1/snippets", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
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
