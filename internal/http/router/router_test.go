package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roguepikachu/bonsai/internal/domain"
	h "github.com/roguepikachu/bonsai/internal/http/handler"
	"github.com/roguepikachu/bonsai/internal/service"
)

// test service implementing handler.SnippetService
type testSvc struct {
	shouldFailCreate bool
	shouldFailList   bool
	shouldFailGet    bool
	snippets         map[string]domain.Snippet
	createdSnippets  []domain.Snippet
}

func (t *testSvc) CreateSnippet(_ context.Context, content string, expiresIn int, tags []string) (domain.Snippet, error) {
	if t.shouldFailCreate {
		return domain.Snippet{}, service.ErrSnippetNotFound
	}
	s := domain.Snippet{
		ID:        "test-id",
		Content:   content,
		Tags:      tags,
		CreatedAt: time.Now(),
	}
	if expiresIn > 0 {
		s.ExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
	}
	if t.snippets == nil {
		t.snippets = make(map[string]domain.Snippet)
	}
	t.snippets[s.ID] = s
	t.createdSnippets = append(t.createdSnippets, s)
	return s, nil
}

func (t *testSvc) ListSnippets(_ context.Context, page int, limit int, tag string) ([]domain.Snippet, error) {
	if t.shouldFailList {
		return nil, service.ErrSnippetNotFound
	}
	if t.snippets == nil {
		return []domain.Snippet{}, nil
	}
	var result []domain.Snippet
	for _, s := range t.snippets {
		result = append(result, s)
	}
	return result, nil
}

func (t *testSvc) GetSnippetByID(_ context.Context, id string) (domain.Snippet, service.SnippetMeta, error) {
	if t.shouldFailGet {
		return domain.Snippet{}, service.SnippetMeta{CacheStatus: service.CacheMiss}, service.ErrSnippetNotFound
	}
	if t.snippets != nil && len(t.snippets) > 0 {
		if s, ok := t.snippets[id]; ok {
			return s, service.SnippetMeta{CacheStatus: service.CacheHit}, nil
		}
	}
	return domain.Snippet{}, service.SnippetMeta{CacheStatus: service.CacheMiss}, service.ErrSnippetNotFound
}

// Mock pinger for health checks
type mockPinger struct {
	err error
}

func (m mockPinger) Ping(_ context.Context) error {
	return m.err
}

func TestNewRouter_RoutesBasic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(h.NewHandler(&testSvc{}), h.NewHealthHandler(nil, nil))

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

func TestRouter_HealthEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// Since NewHealthHandler only accepts real clients, just use nil for basic router testing
	healthHandler := h.NewHealthHandler(nil, nil)
	r := NewRouter(h.NewHandler(&testSvc{}), healthHandler)

	tests := []struct {
		name     string
		path     string
		expected int
	}{
		{"Health", "/v1/health", http.StatusOK},
		{"Liveness", "/v1/livez", http.StatusOK},
		{"Readiness healthy", "/v1/readyz", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if w.Code != tt.expected {
				t.Fatalf("want %d, got %d", tt.expected, w.Code)
			}
		})
	}
}

func TestRouter_SnippetCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &testSvc{}
	r := NewRouter(h.NewHandler(svc), h.NewHealthHandler(nil, nil))

	// Create snippet
	body := `{"content":"test content","expires_in":3600,"tags":["test"]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/snippets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create want 201, got %d", w.Code)
	}

	var createResp domain.SnippetResponseDTO
	if err := json.Unmarshal(w.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to unmarshal create response: %v", err)
	}

	// List snippets
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/snippets", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list want 200, got %d", w.Code)
	}

	var listResp domain.ListSnippetsResponseDTO
	if err := json.Unmarshal(w.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal list response: %v", err)
	}
	if len(listResp.Items) != 1 {
		t.Fatalf("expected 1 snippet, got %d", len(listResp.Items))
	}

	// Get specific snippet
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/snippets/test-id", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("get want 200, got %d", w.Code)
	}

	var getResp domain.SnippetResponseDTO
	if err := json.Unmarshal(w.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("failed to unmarshal get response: %v", err)
	}
	if getResp.Content != "test content" {
		t.Fatalf("expected 'test content', got %s", getResp.Content)
	}
}

func TestRouter_InvalidRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(h.NewHandler(&testSvc{}), h.NewHealthHandler(nil, nil))

	tests := []struct {
		name     string
		method   string
		path     string
		expected int
	}{
		{"Root path", http.MethodGet, "/", http.StatusNotFound},
		{"Invalid path", http.MethodGet, "/invalid", http.StatusNotFound},
		{"Wrong version", http.MethodGet, "/v2/snippets", http.StatusNotFound},
		{"Missing resource", http.MethodGet, "/v1/", http.StatusNotFound},
		{"Wrong method on health", http.MethodDelete, "/v1/health", http.StatusNotFound},       // health only accepts GET
		{"Wrong path structure", http.MethodGet, "/v1/snippets/", http.StatusMovedPermanently}, // gin redirects trailing slash
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))
			if w.Code != tt.expected {
				t.Fatalf("want %d, got %d", tt.expected, w.Code)
			}
		})
	}
}

func TestRouter_MiddlewareOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(h.NewHandler(&testSvc{}), h.NewHealthHandler(nil, nil))

	// Test that middleware is applied correctly
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/health", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	// Check that request ID middleware is working
	if w.Header().Get("X-Request-ID") == "" {
		t.Fatalf("expected X-Request-ID header")
	}
	if w.Header().Get("X-Client-ID") == "" {
		t.Fatalf("expected X-Client-ID header")
	}
}

func TestRouter_ContentTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(h.NewHandler(&testSvc{}), h.NewHealthHandler(nil, nil))

	tests := []struct {
		name        string
		method      string
		path        string
		contentType string
		body        string
		expected    int
	}{
		{"JSON create", http.MethodPost, "/v1/snippets", "application/json", `{"content":"test"}`, http.StatusCreated},
		{"Wrong content type", http.MethodPost, "/v1/snippets", "text/plain", `{"content":"test"}`, http.StatusCreated}, // gin still parses valid JSON
		{"No content type", http.MethodPost, "/v1/snippets", "", `{"content":"test"}`, http.StatusCreated},              // gin still parses valid JSON
		{"Invalid JSON", http.MethodPost, "/v1/snippets", "application/json", `{invalid}`, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			r.ServeHTTP(w, req)
			if w.Code != tt.expected {
				t.Fatalf("want %d, got %d", tt.expected, w.Code)
			}
		})
	}
}

func TestRouter_QueryParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(h.NewHandler(&testSvc{}), h.NewHealthHandler(nil, nil))

	tests := []struct {
		name     string
		path     string
		expected int
	}{
		{"Valid pagination", "/v1/snippets?page=1&limit=10", http.StatusOK},
		{"Valid tag filter", "/v1/snippets?tag=go", http.StatusOK},
		{"Combined params", "/v1/snippets?page=2&limit=5&tag=test", http.StatusOK},
		{"Invalid page", "/v1/snippets?page=0", http.StatusBadRequest},
		{"Invalid limit", "/v1/snippets?limit=0", http.StatusBadRequest},
		{"Large limit", "/v1/snippets?limit=101", http.StatusBadRequest},
		{"Negative page", "/v1/snippets?page=-1", http.StatusBadRequest},
		{"Non-numeric page", "/v1/snippets?page=abc", http.StatusBadRequest},
		{"Non-numeric limit", "/v1/snippets?limit=xyz", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if w.Code != tt.expected {
				t.Fatalf("want %d, got %d", tt.expected, w.Code)
			}
		})
	}
}

func TestRouter_ServiceErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Test with failing service
	failingSvc := &testSvc{
		shouldFailCreate: true,
		shouldFailList:   true,
		shouldFailGet:    true,
	}
	r := NewRouter(h.NewHandler(failingSvc), h.NewHealthHandler(nil, nil))

	tests := []struct {
		name     string
		method   string
		path     string
		body     string
		expected int
	}{
		{"Create fails", http.MethodPost, "/v1/snippets", `{"content":"test"}`, http.StatusInternalServerError},
		{"List fails", http.MethodGet, "/v1/snippets", "", http.StatusInternalServerError},
		{"Get fails", http.MethodGet, "/v1/snippets/test", "", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
				req.Header.Set("Content-Type", "application/json")
			}
			r.ServeHTTP(w, req)
			if w.Code != tt.expected {
				t.Fatalf("want %d, got %d", tt.expected, w.Code)
			}
		})
	}
}

func TestRouter_HTTPMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(h.NewHandler(&testSvc{}), h.NewHealthHandler(nil, nil))

	tests := []struct {
		name     string
		method   string
		path     string
		expected int
	}{
		{"GET snippets", http.MethodGet, "/v1/snippets", http.StatusOK},
		{"POST snippets", http.MethodPost, "/v1/snippets", http.StatusBadRequest}, // no body
		{"PUT not allowed", http.MethodPut, "/v1/snippets", http.StatusNotFound},
		{"DELETE not allowed", http.MethodDelete, "/v1/snippets", http.StatusNotFound},
		{"PATCH not allowed", http.MethodPatch, "/v1/snippets", http.StatusNotFound},
		{"GET snippet by ID", http.MethodGet, "/v1/snippets/test", http.StatusNotFound},
		{"POST on ID not allowed", http.MethodPost, "/v1/snippets/test", http.StatusNotFound},
		{"PUT on ID not allowed", http.MethodPut, "/v1/snippets/test", http.StatusNotFound},
		{"DELETE on ID not allowed", http.MethodDelete, "/v1/snippets/test", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))
			if w.Code != tt.expected {
				t.Fatalf("want %d, got %d", tt.expected, w.Code)
			}
		})
	}
}

func TestRouter_Headers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(h.NewHandler(&testSvc{}), h.NewHealthHandler(nil, nil))

	// Test with custom headers
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	req.Header.Set("X-Request-ID", "test-request-123")
	req.Header.Set("X-Client-ID", "test-client-456")
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("User-Agent", "TestAgent/1.0")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	// Check that request IDs are propagated
	if w.Header().Get("X-Request-ID") != "test-request-123" {
		t.Fatalf("expected X-Request-ID to be propagated")
	}
	if w.Header().Get("X-Client-ID") != "test-client-456" {
		t.Fatalf("expected X-Client-ID to be propagated")
	}
}

func TestRouter_LargePayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(h.NewHandler(&testSvc{}), h.NewHealthHandler(nil, nil))

	// Test with large content
	largeContent := strings.Repeat("a", 10000)
	body := `{"content":"` + largeContent + `","tags":["large"]}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/snippets", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d", w.Code)
	}
}

func TestRouter_ConcurrentRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(h.NewHandler(&testSvc{}), h.NewHealthHandler(nil, nil))

	// Run multiple concurrent requests
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			w := httptest.NewRecorder()
			path := "/v1/health"
			if idx%2 == 0 {
				path = "/v1/snippets"
			}
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
			if w.Code != http.StatusOK {
				t.Errorf("want 200, got %d", w.Code)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestRouter_Panic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a router with recovery middleware
	r := NewRouter(h.NewHandler(&testSvc{}), h.NewHealthHandler(nil, nil))

	// Add a route that panics for testing
	v1 := r.Group("/v1")
	v1.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/panic", nil))

	// Should recover and return 500
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
	}
}
