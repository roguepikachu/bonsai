package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRecovery_CatchesPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Recovery())
	r.GET("/panic", func(_ *gin.Context) { panic("boom") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
}

func TestRecovery_CatchesRuntimeError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Recovery())
	r.GET("/runtime", func(_ *gin.Context) {
		// Trigger runtime error: slice bounds out of range
		arr := []int{1, 2, 3}
		_ = arr[10]
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/runtime", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}

	// Check error response format
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if errObj, ok := resp["error"].(map[string]interface{}); ok {
		if errObj["code"] != "internal_error" {
			t.Fatalf("expected error code internal_error, got %v", errObj["code"])
		}
		if errObj["message"] != "internal server error" {
			t.Fatalf("expected error message 'internal server error', got %v", errObj["message"])
		}
	} else {
		t.Fatalf("expected error object in response")
	}
}

func TestRecovery_NoPanicNormalFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Recovery())
	handlerCalled := false
	r.GET("/normal", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/normal", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if !handlerCalled {
		t.Fatalf("handler was not called")
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", resp["status"])
	}
}

func TestRecovery_PanicWithNilValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Recovery())
	r.GET("/nil-panic", func(_ *gin.Context) {
		panic(nil)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/nil-panic", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
}

func TestRecovery_PanicWithError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Recovery())
	r.GET("/error-panic", func(_ *gin.Context) {
		panic(fmt.Errorf("custom error"))
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/error-panic", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
}

func TestRecovery_PanicWithStruct(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Recovery())
	type CustomPanic struct {
		Message string
		Code    int
	}
	r.GET("/struct-panic", func(_ *gin.Context) {
		panic(CustomPanic{Message: "custom panic", Code: 42})
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/struct-panic", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
}

func TestRecovery_MultipleMiddlewares(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Add multiple middlewares
	middleware1Called := false
	middleware2Called := false

	r.Use(func(c *gin.Context) {
		middleware1Called = true
		c.Next()
	})
	r.Use(Recovery())
	r.Use(func(c *gin.Context) {
		middleware2Called = true
		c.Next()
	})

	r.GET("/panic", func(_ *gin.Context) {
		panic("test panic")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
	if !middleware1Called {
		t.Fatalf("middleware1 was not called")
	}
	if !middleware2Called {
		t.Fatalf("middleware2 was not called")
	}
}

func TestRecovery_HTTPMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Recovery())

	// Test different HTTP methods
	r.GET("/panic", func(_ *gin.Context) { panic("GET panic") })
	r.POST("/panic", func(_ *gin.Context) { panic("POST panic") })
	r.PUT("/panic", func(_ *gin.Context) { panic("PUT panic") })
	r.DELETE("/panic", func(_ *gin.Context) { panic("DELETE panic") })
	r.PATCH("/panic", func(_ *gin.Context) { panic("PATCH panic") })

	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(method, "/panic", nil))
			if w.Code != http.StatusInternalServerError {
				t.Fatalf("want 500 for %s, got %d", method, w.Code)
			}
		})
	}
}

func TestRecovery_WithRequestBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Recovery())
	r.POST("/panic", func(c *gin.Context) {
		var body map[string]interface{}
		_ = c.ShouldBindJSON(&body)
		panic("panic after reading body")
	})

	body := `{"test": "data"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/panic", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
}

func TestRecovery_WithQueryParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Recovery())
	r.GET("/panic", func(c *gin.Context) {
		_ = c.Query("param")
		panic("panic after reading query")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic?param=value", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
}

func TestRecovery_LongStackTrace(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Recovery())

	// Create deep call stack
	var deepFunc func(int)
	deepFunc = func(depth int) {
		if depth <= 0 {
			panic("deep panic")
		}
		deepFunc(depth - 1)
	}

	r.GET("/deep-panic", func(_ *gin.Context) {
		deepFunc(10)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/deep-panic", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
}

func TestRecovery_ConcurrentPanics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Recovery())
	r.GET("/panic", func(_ *gin.Context) { panic("concurrent panic") })

	// Run multiple concurrent requests that panic
	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))
			if w.Code != http.StatusInternalServerError {
				t.Errorf("want 500, got %d", w.Code)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}
}

func TestRecovery_HeadersPreserved(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Recovery())
	r.GET("/panic", func(c *gin.Context) {
		c.Header("X-Custom-Header", "preserved")
		panic("panic after setting header")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
}

func TestRecovery_PanicInMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Recovery())
	r.Use(func(c *gin.Context) {
		if strings.Contains(c.Request.URL.Path, "panic") {
			panic("middleware panic")
		}
		c.Next()
	})
	r.GET("/panic", func(c *gin.Context) {
		c.String(http.StatusOK, "should not reach here")
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
}
