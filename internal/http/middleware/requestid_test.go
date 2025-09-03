package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roguepikachu/bonsai/internal/utils"
)

func TestRequestIDMiddleware_SetsHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestIDMiddleware())
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if got := w.Header().Get(headerRequestID); got == "" {
		t.Fatalf("%s header should be set", headerRequestID)
	}
	if got := w.Header().Get(headerClientID); got == "" {
		t.Fatalf("%s header should be set", headerClientID)
	}
}

func TestRequestIDMiddleware_PropagatesProvided(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestIDMiddleware())
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("X-Request-ID", "rid-xyz")
	req.Header.Set("X-Client-ID", "cid-xyz")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Header().Get(headerRequestID) != "rid-xyz" || w.Header().Get(headerClientID) != "cid-xyz" {
		t.Fatalf("did not propagate provided headers: %s %s", w.Header().Get(headerRequestID), w.Header().Get(headerClientID))
	}
}

func TestRequestIDMiddleware_GeneratesNewIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestIDMiddleware())
	r.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// Make multiple requests without IDs
	ids := make(map[string]bool)
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("want 200, got %d", w.Code)
		}

		requestID := w.Header().Get(headerRequestID)
		clientID := w.Header().Get(headerClientID)

		if requestID == "" || clientID == "" {
			t.Fatalf("IDs should be generated")
		}

		// Check for uniqueness
		if ids[requestID] {
			t.Fatalf("duplicate request ID: %s", requestID)
		}
		ids[requestID] = true

		// Verify UUID format (36 chars with hyphens)
		if len(requestID) != 36 {
			t.Fatalf("expected UUID format (36 chars), got %d: %s", len(requestID), requestID)
		}
		if len(clientID) != 36 {
			t.Fatalf("expected UUID format (36 chars), got %d: %s", len(clientID), clientID)
		}
	}
}

func TestRequestIDMiddleware_OnlyRequestIDProvided(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestIDMiddleware())
	r.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "provided-request-id")
	// Not setting X-Client-ID
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	requestID := w.Header().Get(headerRequestID)
	clientID := w.Header().Get(headerClientID)

	if requestID != "provided-request-id" {
		t.Fatalf("expected provided request ID, got %s", requestID)
	}
	if clientID == "" {
		t.Fatalf("client ID should be generated when not provided")
	}
	if len(clientID) != 36 {
		t.Fatalf("expected generated UUID for client ID")
	}
}

func TestRequestIDMiddleware_OnlyClientIDProvided(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestIDMiddleware())
	r.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Client-ID", "provided-client-id")
	// Not setting X-Request-ID
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	requestID := w.Header().Get(headerRequestID)
	clientID := w.Header().Get(headerClientID)

	if clientID != "provided-client-id" {
		t.Fatalf("expected provided client ID, got %s", clientID)
	}
	if requestID == "" {
		t.Fatalf("request ID should be generated when not provided")
	}
	if len(requestID) != 36 {
		t.Fatalf("expected generated UUID for request ID")
	}
}

func TestRequestIDMiddleware_ContextValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestIDMiddleware())

	var contextRequestID, contextClientID string
	r.GET("/test", func(c *gin.Context) {
		contextRequestID = utils.RequestID(c.Request.Context())
		contextClientID = utils.ClientID(c.Request.Context())
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "ctx-request-id")
	req.Header.Set("X-Client-ID", "ctx-client-id")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	if contextRequestID != "ctx-request-id" {
		t.Fatalf("expected request ID in context, got %s", contextRequestID)
	}
	if contextClientID != "ctx-client-id" {
		t.Fatalf("expected client ID in context, got %s", contextClientID)
	}
}

func TestRequestIDMiddleware_HTTPMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestIDMiddleware())

	r.GET("/resource", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.POST("/resource", func(c *gin.Context) { c.Status(http.StatusCreated) })
	r.PUT("/resource", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.DELETE("/resource", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	r.PATCH("/resource", func(c *gin.Context) { c.Status(http.StatusOK) })

	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/resource", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			requestID := w.Header().Get(headerRequestID)
			clientID := w.Header().Get(headerClientID)
			if requestID == "" || clientID == "" {
				t.Fatalf("IDs should be set for %s method", method)
			}
		})
	}
}

func TestRequestIDMiddleware_EmptyStringHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestIDMiddleware())
	r.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// Set empty string headers
	req.Header.Set("X-Request-ID", "")
	req.Header.Set("X-Client-ID", "")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	requestID := w.Header().Get(headerRequestID)
	clientID := w.Header().Get(headerClientID)

	// Should generate new IDs when empty
	if requestID == "" || clientID == "" {
		t.Fatalf("IDs should be generated when empty strings provided")
	}
	if len(requestID) != 36 || len(clientID) != 36 {
		t.Fatalf("expected UUID format for generated IDs")
	}
}

func TestRequestIDMiddleware_WhitespaceHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestIDMiddleware())
	r.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// Set whitespace headers
	req.Header.Set("X-Request-ID", "   ")
	req.Header.Set("X-Client-ID", "\t\n")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	requestID := w.Header().Get(headerRequestID)
	clientID := w.Header().Get(headerClientID)

	// Should use the whitespace as-is (gin doesn't trim)
	if requestID != "   " || clientID != "\t\n" {
		t.Fatalf("expected whitespace to be preserved, got %q and %q", requestID, clientID)
	}
}

func TestRequestIDMiddleware_LongHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestIDMiddleware())
	r.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// Create very long IDs
	longID := strings.Repeat("a", 1000)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", longID)
	req.Header.Set("X-Client-ID", longID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	requestID := w.Header().Get(headerRequestID)
	clientID := w.Header().Get(headerClientID)

	if requestID != longID || clientID != longID {
		t.Fatalf("long IDs should be preserved")
	}
}

func TestRequestIDMiddleware_SpecialCharacters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestIDMiddleware())
	r.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// Test with special characters
	specialID := "test-123_ABC.xyz~!@#$%^&*()"
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", specialID)
	req.Header.Set("X-Client-ID", specialID)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	requestID := w.Header().Get(headerRequestID)
	clientID := w.Header().Get(headerClientID)

	if requestID != specialID || clientID != specialID {
		t.Fatalf("special characters should be preserved")
	}
}

func TestRequestIDMiddleware_MultipleMiddlewares(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	var firstRequestID, firstClientID string
	var secondRequestID, secondClientID string

	// First middleware to capture IDs
	r.Use(func(c *gin.Context) {
		c.Next()
		firstRequestID = c.Writer.Header().Get(headerRequestID)
		firstClientID = c.Writer.Header().Get(headerClientID)
	})

	r.Use(RequestIDMiddleware())

	// Second middleware to capture IDs
	r.Use(func(c *gin.Context) {
		secondRequestID = c.Writer.Header().Get(headerRequestID)
		secondClientID = c.Writer.Header().Get(headerClientID)
		c.Next()
	})

	r.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "test-request")
	req.Header.Set("X-Client-ID", "test-client")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	// Both middlewares should see the same IDs
	if firstRequestID != "test-request" || firstClientID != "test-client" {
		t.Fatalf("first middleware got wrong IDs: %s, %s", firstRequestID, firstClientID)
	}
	if secondRequestID != "test-request" || secondClientID != "test-client" {
		t.Fatalf("second middleware got wrong IDs: %s, %s", secondRequestID, secondClientID)
	}
}

func TestRequestIDMiddleware_ConcurrentRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestIDMiddleware())
	r.GET("/test", func(c *gin.Context) {
		// Verify context has IDs
		if utils.RequestID(c.Request.Context()) == "" {
			t.Errorf("no request ID in context")
		}
		if utils.ClientID(c.Request.Context()) == "" {
			t.Errorf("no client ID in context")
		}
		c.String(http.StatusOK, "ok")
	})

	// Run multiple concurrent requests
	done := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		go func(idx int) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if idx%2 == 0 {
				// Half with provided IDs
				req.Header.Set("X-Request-ID", strings.Repeat("r", idx+1))
				req.Header.Set("X-Client-ID", strings.Repeat("c", idx+1))
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("want 200, got %d", w.Code)
			}

			requestID := w.Header().Get(headerRequestID)
			clientID := w.Header().Get(headerClientID)
			if requestID == "" || clientID == "" {
				t.Errorf("IDs should be set")
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestRequestIDMiddleware_WithRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestIDMiddleware())
	r.POST("/test", func(c *gin.Context) {
		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"request_id": utils.RequestID(c.Request.Context()),
			"client_id":  utils.ClientID(c.Request.Context()),
			"body":       body,
		})
	})

	body := `{"test": "data"}`
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["request_id"] == "" || resp["client_id"] == "" {
		t.Fatalf("IDs should be in response")
	}
}

func TestUtils_RequestID_EmptyContext(t *testing.T) {
	// Test with empty context
	id := utils.RequestID(context.Background())
	if id != "" {
		t.Fatalf("expected empty string for empty context, got %s", id)
	}
}

func TestUtils_ClientID_EmptyContext(t *testing.T) {
	// Test with empty context
	id := utils.ClientID(context.Background())
	if id != "" {
		t.Fatalf("expected empty string for empty context, got %s", id)
	}
}
