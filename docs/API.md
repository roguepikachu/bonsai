# API.md

All endpoints are **JSON only** and versioned under `/v1`.

Error responses follow this envelope:

```json
{
  "error": {
    "code": "string",
    "message": "string",
    "details": { }
  }
}
```

---

## Endpoints

### 1. Health

**GET /v1/health**

Checks service and Redis connectivity.

**200 Response**

```json
{ "ok": true, "redis": "pong" }
```

---

### 2. Create Snippet

**POST /v1/snippets**

Create a new snippet with optional expiry and tags.

**Request**

```json
{
  "content": "hello world",
  "expires_in": 86400,
  "tags": ["demo", "test"]
}
```

**201 Response**

```json
{
  "id": "abc123",
  "created_at": "2025-08-21T15:04:05Z",
  "expires_at": "2025-08-22T15:04:05Z",
  "tags": ["demo", "test"]
}
```

**Errors**

* 400 if content > 10KB
* 400 if expires\_in > 30 days

---

### 3. List Snippets

**GET /v1/snippets**

List all snippets with pagination and optional tag filter.

**Query Params**

* `page` (default 1)
* `limit` (default 20)
* `tag` (optional filter)

**200 Response**

```json
{
  "page": 1,
  "limit": 2,
  "items": [
    { "id": "abc123", "created_at": "2025-08-21T15:04:05Z", "expires_at": null },
    { "id": "def456", "created_at": "2025-08-21T15:05:05Z", "expires_at": "2025-08-22T15:05:05Z" }
  ]
}
```

---

### 4. Get Snippet by ID

**GET /v1/snippets/\:id**

Fetches snippet content. Uses cache-aside in Redis.

**200 Response**

```json
{
  "id": "abc123",
  "content": "hello world",
  "created_at": "2025-08-21T15:04:05Z",
  "expires_at": null
}
```

**Headers**

* `X-Cache: HIT` or `MISS`

**Errors**

* 404 if not found
* 410 if expired

---

### 5. Update Snippet

**PATCH /v1/snippets/\:id**

Updates snippet content or expiry. Invalidates cache.

**Request**

```json
{ "content": "new text", "expires_in": 172800 }
```

**200 Response**

```json
{
  "id": "abc123",
  "content": "new text",
  "expires_at": "2025-08-23T15:04:05Z"
}
```

**Errors**

* 404 if not found
* 400 for invalid fields

---

### 6. Delete Snippet

**DELETE /v1/snippets/\:id**

Soft delete a snippet and clears cache.

**200 Response**

```json
{ "deleted": true }
```

**Errors**

* 404 if not found

---

### 7. Snippet Metrics

**GET /v1/snippets/\:id/metrics**

Returns aggregated analytics.

**200 Response**

```json
{
  "id": "abc123",
  "total_views": 152,
  "unique_visitors": 89,
  "daily": [
    { "date": "2025-08-19", "views": 50, "uniques": 30 },
    { "date": "2025-08-20", "views": 102, "uniques": 59 }
  ],
  "trend_score": 74.2
}
```

**Errors**

* 404 if snippet not found

---

### 8. Service Metrics

**GET /v1/metrics**

Prometheus-style metrics for ops and monitoring.

**Sample Response**

```
http_requests_total{route="/v1/snippets"} 523
cache_hits_total 420
cache_misses_total 103
ratelimit_block_total 2
events_processed_total 510
events_dlq_total 1
```

---

## Mermaid Diagram of Endpoints

```mermaid
flowchart TB
  A["GET /v1/health"]
  B["POST /v1/snippets"]
  C["GET /v1/snippets"]
  D["GET /v1/snippets/:id"]
  E["PATCH /v1/snippets/:id"]
  F["DELETE /v1/snippets/:id"]
  G["GET /v1/snippets/:id/metrics"]
  H["GET /v1/metrics"]

  A --> B
  B --> C
  C --> D
  D --> E
  E --> F
  D --> G
  A --> H
```
