package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRequestLogger_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/ok", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

func TestRequestLogger_4xx(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/bad", func(c *gin.Context) { c.String(http.StatusBadRequest, "bad") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/bad", nil))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestRequestLogger_5xx(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/err", func(c *gin.Context) { c.String(http.StatusInternalServerError, "err") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/err", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
}

func TestRequestLogger_AllStatusCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name   string
		status int
	}{
		{"OK", http.StatusOK},
		{"Created", http.StatusCreated},
		{"NoContent", http.StatusNoContent},
		{"MovedPermanently", http.StatusMovedPermanently},
		{"NotModified", http.StatusNotModified},
		{"BadRequest", http.StatusBadRequest},
		{"Unauthorized", http.StatusUnauthorized},
		{"Forbidden", http.StatusForbidden},
		{"NotFound", http.StatusNotFound},
		{"MethodNotAllowed", http.StatusMethodNotAllowed},
		{"Conflict", http.StatusConflict},
		{"Gone", http.StatusGone},
		{"UnprocessableEntity", http.StatusUnprocessableEntity},
		{"TooManyRequests", http.StatusTooManyRequests},
		{"InternalServerError", http.StatusInternalServerError},
		{"BadGateway", http.StatusBadGateway},
		{"ServiceUnavailable", http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.Use(RequestLogger())
			r.GET("/test", func(c *gin.Context) { c.Status(tt.status) })

			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))

			if w.Code != tt.status {
				t.Fatalf("want %d, got %d", tt.status, w.Code)
			}
		})
	}
}

func TestRequestLogger_HTTPMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogger())
	
	r.GET("/resource", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/resource", func(c *gin.Context) { c.Status(http.StatusCreated) })
	r.PUT("/resource", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.DELETE("/resource", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	r.PATCH("/resource", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.HEAD("/resource", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.OPTIONS("/resource", func(c *gin.Context) { c.Status(http.StatusOK) })

	tests := []struct {
		method         string
		expectedStatus int
	}{
		{http.MethodGet, http.StatusOK},
		{http.MethodPost, http.StatusCreated},
		{http.MethodPut, http.StatusOK},
		{http.MethodDelete, http.StatusNoContent},
		{http.MethodPatch, http.StatusOK},
		{http.MethodHead, http.StatusOK},
		{http.MethodOptions, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tt.method, "/resource", nil))
			if w.Code != tt.expectedStatus {
				t.Fatalf("want %d for %s, got %d", tt.expectedStatus, tt.method, w.Code)
			}
		})
	}
}

func TestRequestLogger_WithQueryParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/search", func(c *gin.Context) {
		q := c.Query("q")
		limit := c.Query("limit")
		c.JSON(http.StatusOK, gin.H{"query": q, "limit": limit})
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/search?q=test&limit=10", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

func TestRequestLogger_WithRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogger())
	r.POST("/data", func(c *gin.Context) {
		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, body)
	})

	body := `{"key": "value", "number": 42}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/data", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

func TestRequestLogger_WithHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/headers", func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		userAgent := c.GetHeader("User-Agent")
		c.JSON(http.StatusOK, gin.H{"auth": auth, "user_agent": userAgent})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/headers", nil)
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("User-Agent", "TestAgent/1.0")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

func TestRequestLogger_SlowRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/slow", func(c *gin.Context) {
		time.Sleep(100 * time.Millisecond)
		c.String(http.StatusOK, "slow response")
	})

	start := time.Now()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/slow", nil))
	elapsed := time.Since(start)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if elapsed < 100*time.Millisecond {
		t.Fatalf("request should have taken at least 100ms, took %v", elapsed)
	}
}

func TestRequestLogger_LargeResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/large", func(c *gin.Context) {
		// Generate large response
		data := make([]byte, 1024*1024) // 1MB
		for i := range data {
			data[i] = byte(i % 256)
		}
		c.Data(http.StatusOK, "application/octet-stream", data)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/large", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if w.Body.Len() != 1024*1024 {
		t.Fatalf("expected 1MB response, got %d bytes", w.Body.Len())
	}
}

func TestRequestLogger_RouteNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/exists", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/not-found", nil))

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestRequestLogger_MethodNotAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/resource", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/resource", nil))

	// Gin returns 404 for method not allowed by default
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestRequestLogger_MultipleMiddlewares(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	
	middleware1Called := false
	middleware2Called := false
	
	r.Use(func(c *gin.Context) {
		middleware1Called = true
		c.Next()
	})
	r.Use(RequestLogger())
	r.Use(func(c *gin.Context) {
		middleware2Called = true
		c.Next()
	})
	
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if !middleware1Called {
		t.Fatalf("middleware1 was not called")
	}
	if !middleware2Called {
		t.Fatalf("middleware2 was not called")
	}
}

func TestRequestLogger_ConcurrentRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/concurrent", func(c *gin.Context) {
		time.Sleep(10 * time.Millisecond)
		c.String(http.StatusOK, "ok")
	})

	// Run multiple concurrent requests
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/concurrent", nil))
			if w.Code != http.StatusOK {
				t.Errorf("want 200, got %d", w.Code)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestRequestLogger_WithPathParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/users/:id", func(c *gin.Context) {
		id := c.Param("id")
		c.JSON(http.StatusOK, gin.H{"user_id": id})
	})
	r.GET("/users/:id/posts/:postId", func(c *gin.Context) {
		id := c.Param("id")
		postId := c.Param("postId")
		c.JSON(http.StatusOK, gin.H{"user_id": id, "post_id": postId})
	})

	// Test single param
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/users/123", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["user_id"] != "123" {
		t.Fatalf("expected user_id 123, got %v", resp["user_id"])
	}

	// Test multiple params
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/users/456/posts/789", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["user_id"] != "456" || resp["post_id"] != "789" {
		t.Fatalf("expected user_id 456 and post_id 789, got %v", resp)
	}
}

func TestRequestLogger_EmptyResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/empty", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/empty", nil))

	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %d bytes", w.Body.Len())
	}
}

func TestRequestLogger_Redirect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/old", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/new")
	})
	r.GET("/new", func(c *gin.Context) {
		c.String(http.StatusOK, "new location")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/old", nil))

	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("want 301, got %d", w.Code)
	}
	if location := w.Header().Get("Location"); location != "/new" {
		t.Fatalf("expected Location header /new, got %s", location)
	}
}

func TestRequestLogger_CustomContentTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestLogger())
	
	r.GET("/json", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"type": "json"})
	})
	r.GET("/xml", func(c *gin.Context) {
		c.XML(http.StatusOK, gin.H{"type": "xml"})
	})
	r.GET("/text", func(c *gin.Context) {
		c.String(http.StatusOK, "plain text")
	})
	r.GET("/html", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html", []byte("<h1>HTML</h1>"))
	})

	tests := []struct {
		path        string
		contentType string
	}{
		{"/json", "application/json"},
		{"/xml", "application/xml"},
		{"/text", "text/plain"},
		{"/html", "text/html"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if w.Code != http.StatusOK {
				t.Fatalf("want 200, got %d", w.Code)
			}
			contentType := w.Header().Get("Content-Type")
			if !strings.Contains(contentType, tt.contentType) {
				t.Fatalf("expected Content-Type %s, got %s", tt.contentType, contentType)
			}
		})
	}
}
