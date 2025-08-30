package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// fake pgxpool with Ping override
type fakePinger struct{ err error }

func (f fakePinger) Ping(ctx context.Context) error { return f.err }

func TestLiveness_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hh := &HealthHandler{pingTimeout: time.Second}
	r := gin.New()
	r.GET("/v1/livez", hh.Liveness)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/livez", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

func TestReadiness_AllUp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hh := &HealthHandler{pg: fakePinger{}, redis: fakePinger{}, pingTimeout: time.Second}
	r := gin.New()
	r.GET("/v1/readyz", hh.Readiness)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/readyz", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
}

func TestReadiness_FailDeps(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hh := &HealthHandler{pingTimeout: time.Second}
	hh.pg = fakePinger{}
	hh.redis = fakePinger{err: errors.New("redis down")}

	r := gin.New()
	r.GET("/v1/readyz", hh.Readiness)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/readyz", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", w.Code)
	}
}
