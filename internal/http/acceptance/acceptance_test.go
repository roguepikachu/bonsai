package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roguepikachu/bonsai/internal/domain"
	httpHandlers "github.com/roguepikachu/bonsai/internal/http/handler"
	appRouter "github.com/roguepikachu/bonsai/internal/http/router"
	fakeRepo "github.com/roguepikachu/bonsai/internal/repository/fake"
	"github.com/roguepikachu/bonsai/internal/service"
)

// fixedClock implements service.Clock for deterministic timestamps in tests.
type fixedClock struct{ t time.Time }

func (f fixedClock) Now() time.Time { return f.t }

// newTestServer constructs a complete HTTP server with handlers wired to a fake repo.
func newTestServer(t *testing.T, repo *fakeRepo.SnippetRepository, clk service.Clock, idGen func() string) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// Build service with deterministic ID if provided.
	opts := []service.Option{}
	if idGen != nil {
		opts = append(opts, service.WithIDGenerator(idGen))
	}
	svc := service.NewServiceWithOptions(repo, clk, opts...)
	snipHandler := httpHandlers.NewHandler(svc)

	// Pass nil deps; readiness will be considered ready with no checks.
	health := httpHandlers.NewHealthHandler(nil, nil)

	r := appRouter.NewRouter(snipHandler, health)
	return httptest.NewServer(r)
}

// helper to perform HTTP requests and decode JSON into v.
func doJSON(t *testing.T, client *http.Client, method, url string, body any, v any) (int, http.Header) {
	t.Helper()
	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		buf = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if v != nil {
		dec := json.NewDecoder(resp.Body)
		if err := dec.Decode(v); err != nil {
			t.Fatalf("decode: %v", err)
		}
	} else {
		_, _ = io.Copy(io.Discard, resp.Body)
	}
	return resp.StatusCode, resp.Header
}

func TestHealthEndpoints(t *testing.T) {
	// Minimal repo/clock just to build router
	base := time.Date(2025, 8, 31, 10, 0, 0, 0, time.UTC)
	repo := fakeRepo.NewSnippetRepository(fakeRepo.WithNow(func() time.Time { return base }))
	srv := newTestServer(t, repo, fixedClock{t: base}, nil)
	defer srv.Close()

	client := srv.Client()

	// /v1/health
	var healthResp struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
		Msg  string         `json:"message"`
	}
	code, _ := doJSON(t, client, http.MethodGet, srv.URL+"/v1/health", nil, &healthResp)
	if code != http.StatusOK || healthResp.Code != 200 || healthResp.Data["ok"] != true {
		t.Fatalf("/health failed: code=%d resp=%+v", code, healthResp)
	}

	// /v1/livez
	var liveResp struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	code, _ = doJSON(t, client, http.MethodGet, srv.URL+"/v1/livez", nil, &liveResp)
	if code != http.StatusOK || liveResp.Data["status"] != "alive" {
		t.Fatalf("/livez failed: code=%d resp=%+v", code, liveResp)
	}

	// /v1/readyz healthy
	var readyResp struct {
		Code int `json:"code"`
		Data struct {
			Ready  bool             `json:"ready"`
			Checks []map[string]any `json:"checks"`
		} `json:"data"`
	}
	code, _ = doJSON(t, client, http.MethodGet, srv.URL+"/v1/readyz", nil, &readyResp)
	if code != http.StatusOK || !readyResp.Data.Ready {
		t.Fatalf("/readyz healthy failed: code=%d resp=%+v", code, readyResp)
	}

	// Unhealthy scenario not covered here because HealthHandler hides its pingers; covered via integration tests.
}

func TestSnippetCRUDLikeEndpoints(t *testing.T) {
	base := time.Date(2025, 8, 31, 12, 34, 56, 0, time.UTC)
	repo := fakeRepo.NewSnippetRepository(fakeRepo.WithNow(func() time.Time { return base }))
	idseq := 0
	nextID := func() string { idseq++; return "id-" + string(rune('a'+idseq-1)) }
	srv := newTestServer(t, repo, fixedClock{t: base}, nextID)
	defer srv.Close()
	client := srv.Client()

	// Create
	req := map[string]any{"content": "hello world", "expires_in": 60, "tags": []string{"demo"}}
	var created struct {
		ID        string   `json:"id"`
		Content   string   `json:"content"`
		CreatedAt string   `json:"created_at"`
		ExpiresAt *string  `json:"expires_at"`
		Tags      []string `json:"tags"`
	}
	code, _ := doJSON(t, client, http.MethodPost, srv.URL+"/v1/snippets", req, &created)
	if code != http.StatusCreated || created.ID == "" || created.Content != "hello world" {
		t.Fatalf("create failed: code=%d resp=%+v", code, created)
	}
	if created.CreatedAt != base.Format(httpHandlers.TimeFormat) {
		t.Fatalf("unexpected created_at: %s", created.CreatedAt)
	}
	wantExp := base.Add(60 * time.Second).Format(httpHandlers.TimeFormat)
	if created.ExpiresAt == nil || *created.ExpiresAt != wantExp {
		t.Fatalf("unexpected expires_at: %v want %s", created.ExpiresAt, wantExp)
	}

	// Get by ID
	var got struct {
		ID        string  `json:"id"`
		Content   string  `json:"content"`
		CreatedAt string  `json:"created_at"`
		ExpiresAt *string `json:"expires_at"`
	}
	code, hdr := doJSON(t, client, http.MethodGet, srv.URL+"/v1/snippets/"+created.ID, nil, &got)
	if code != http.StatusOK || got.ID != created.ID || got.Content != "hello world" {
		t.Fatalf("get failed: code=%d resp=%+v", code, got)
	}
	if hdr.Get("X-Cache") == "" {
		t.Fatalf("missing X-Cache header")
	}

	// List
	var list struct {
		Page  int `json:"page"`
		Limit int `json:"limit"`
		Items []struct {
			ID        string  `json:"id"`
			CreatedAt string  `json:"created_at"`
			ExpiresAt *string `json:"expires_at"`
		} `json:"items"`
	}
	code, _ = doJSON(t, client, http.MethodGet, srv.URL+"/v1/snippets?page=1&limit=10", nil, &list)
	if code != http.StatusOK || len(list.Items) < 1 {
		t.Fatalf("list failed: code=%d items=%d", code, len(list.Items))
	}

	// Validation: invalid expires_in
	badReq := map[string]any{"content": "oops", "expires_in": -1}
	var errResp map[string]any
	code, _ = doJSON(t, client, http.MethodPost, srv.URL+"/v1/snippets", badReq, &errResp)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d resp=%+v", code, errResp)
	}

	// Not found
	code, _ = doJSON(t, client, http.MethodGet, srv.URL+"/v1/snippets/does-not-exist", nil, &errResp)
	if code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", code)
	}

	// Expired snippet -> 410
	// Insert directly into repo with past expiry
	past := base.Add(-time.Minute)
	_ = repo.Insert(context.TODO(), domain.Snippet{ID: "old", Content: "zzz", CreatedAt: base.Add(-2 * time.Minute), ExpiresAt: past})
	code, _ = doJSON(t, client, http.MethodGet, srv.URL+"/v1/snippets/old", nil, &errResp)
	if code != http.StatusGone {
		t.Fatalf("expected 410, got %d", code)
	}
}

func TestList_Defaults_TagFilter_Pagination(t *testing.T) {
	base := time.Date(2025, 8, 31, 13, 0, 0, 0, time.UTC)
	// Seed repo with mixed tags and an expired item; ensure ordering
	repo := fakeRepo.NewSnippetRepository(fakeRepo.WithNow(func() time.Time { return base }))
	// Non-expired
	_ = repo.Insert(context.TODO(), domain.Snippet{ID: "s1", Content: "a", Tags: []string{"go", "demo"}, CreatedAt: base.Add(1 * time.Second)})
	_ = repo.Insert(context.TODO(), domain.Snippet{ID: "s2", Content: "b", Tags: []string{"rust"}, CreatedAt: base.Add(2 * time.Second)})
	_ = repo.Insert(context.TODO(), domain.Snippet{ID: "s3", Content: "c", Tags: []string{"Go"}, CreatedAt: base.Add(3 * time.Second)})
	// Expired (should not appear)
	_ = repo.Insert(context.TODO(), domain.Snippet{ID: "old", Content: "z", Tags: []string{"go"}, CreatedAt: base.Add(-10 * time.Second), ExpiresAt: base.Add(-1 * time.Second)})

	srv := newTestServer(t, repo, fixedClock{t: base}, nil)
	defer srv.Close()
	client := srv.Client()

	// Defaults (page=1, limit=20)
	var list struct {
		Page  int `json:"page"`
		Limit int `json:"limit"`
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	code, _ := doJSON(t, client, http.MethodGet, srv.URL+"/v1/snippets", nil, &list)
	if code != http.StatusOK || list.Page != 1 || list.Limit != 20 || len(list.Items) != 3 {
		t.Fatalf("defaults or count wrong: code=%d page=%d limit=%d items=%d", code, list.Page, list.Limit, len(list.Items))
	}
	// Ordering (desc by created_at): s3, s2, s1
	if len(list.Items) < 3 || list.Items[0].ID != "s3" || list.Items[1].ID != "s2" || list.Items[2].ID != "s1" {
		t.Fatalf("unexpected order: %+v", list.Items)
	}

	// Tag filter (case-insensitive "go"): should return s3 then s1
	code, _ = doJSON(t, client, http.MethodGet, srv.URL+"/v1/snippets?tag=go", nil, &list)
	if code != http.StatusOK || len(list.Items) != 2 || list.Items[0].ID != "s3" || list.Items[1].ID != "s1" {
		t.Fatalf("tag filter wrong: code=%d items=%v", code, list.Items)
	}

	// Pagination: limit=2 -> page1 two items, page2 one item
	code, _ = doJSON(t, client, http.MethodGet, srv.URL+"/v1/snippets?limit=2", nil, &list)
	if code != http.StatusOK || len(list.Items) != 2 {
		t.Fatalf("page1 wrong: items=%d", len(list.Items))
	}
	code, _ = doJSON(t, client, http.MethodGet, srv.URL+"/v1/snippets?limit=2&page=2", nil, &list)
	if code != http.StatusOK || len(list.Items) != 1 || list.Items[0].ID != "s1" {
		t.Fatalf("page2 wrong: %+v", list.Items)
	}

	// Bad query params -> 400 (due to binding rules)
	var errResp map[string]any
	code, _ = doJSON(t, client, http.MethodGet, srv.URL+"/v1/snippets?limit=0", nil, &errResp)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400 for limit=0, got %d", code)
	}
	code, _ = doJSON(t, client, http.MethodGet, srv.URL+"/v1/snippets?page=0", nil, &errResp)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400 for page=0, got %d", code)
	}
	code, _ = doJSON(t, client, http.MethodGet, srv.URL+"/v1/snippets?limit=101", nil, &errResp)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400 for limit>100, got %d", code)
	}
}

func TestCreate_ValidationAndNoExpiry_TagsOptional(t *testing.T) {
	base := time.Date(2025, 8, 31, 14, 0, 0, 0, time.UTC)
	repo := fakeRepo.NewSnippetRepository(fakeRepo.WithNow(func() time.Time { return base }))
	srv := newTestServer(t, repo, fixedClock{t: base}, nil)
	defer srv.Close()
	client := srv.Client()

	// Content too large (10KB+1)
	big := bytes.Repeat([]byte{'a'}, 10241)
	var errResp map[string]any
	code, _ := doJSON(t, client, http.MethodPost, srv.URL+"/v1/snippets", map[string]any{"content": string(big)}, &errResp)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized content, got %d", code)
	}

	// expires_in too large (>30 days)
	code, _ = doJSON(t, client, http.MethodPost, srv.URL+"/v1/snippets", map[string]any{"content": "ok", "expires_in": 2592001}, &errResp)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400 for large expires_in, got %d", code)
	}

	// No expiry (expires_in=0) and tags omitted
	var created map[string]any
	code, _ = doJSON(t, client, http.MethodPost, srv.URL+"/v1/snippets", map[string]any{"content": "ok", "expires_in": 0}, &created)
	if code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", code)
	}
	if _, ok := created["expires_at"]; ok {
		t.Fatalf("expires_at should be omitted when no expiry: %+v", created)
	}
	// tags field may be omitted; if present and empty, it's fine either way
}

func TestHeaders_Propagation_RequestAndClientID(t *testing.T) {
	base := time.Date(2025, 8, 31, 15, 0, 0, 0, time.UTC)
	repo := fakeRepo.NewSnippetRepository(fakeRepo.WithNow(func() time.Time { return base }))
	srv := newTestServer(t, repo, fixedClock{t: base}, nil)
	defer srv.Close()

	// With provided headers
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/health", nil)
	req.Header.Set("X-Request-ID", "rid-123")
	req.Header.Set("X-Client-ID", "cid-456")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	_ = resp.Body.Close()
	if resp.Header.Get("X-Request-ID") != "rid-123" || resp.Header.Get("X-Client-ID") != "cid-456" {
		t.Fatalf("header propagation failed: %s %s", resp.Header.Get("X-Request-ID"), resp.Header.Get("X-Client-ID"))
	}

	// Without provided headers — should be auto-populated
	req2, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/health", nil)
	resp2, err := srv.Client().Do(req2)
	if err != nil {
		t.Fatalf("do2: %v", err)
	}
	_ = resp2.Body.Close()
	if resp2.Header.Get("X-Request-ID") == "" || resp2.Header.Get("X-Client-ID") == "" {
		t.Fatalf("missing auto headers: %v %v", resp2.Header.Get("X-Request-ID"), resp2.Header.Get("X-Client-ID"))
	}
}
