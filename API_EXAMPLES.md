# Bonsai API Examples

This document contains comprehensive examples of all API endpoints with actual requests and responses captured from the running Bonsai API server.

## Base URL
```
http://localhost:8080
```

## API Endpoints Overview

### Health Endpoints
- `GET /v1/livez` - Liveness check
- `GET /v1/readyz` - Readiness check

### Snippet Operations
- `POST /v1/snippets` - Create a new snippet
- `GET /v1/snippets` - List snippets with pagination and filtering
- `GET /v1/snippets/{id}` - Get a specific snippet
- `PUT /v1/snippets/{id}` - Update a snippet
- `DELETE /v1/snippets/{id}` - Delete a snippet

---

## Detailed Examples

### 1. Health Check - Liveness

**Request:**
```bash
GET /v1/livez
```

**Response:**
```json
{
  "code": 200,
  "data": {
    "status": "alive"
  },
  "message": "ok"
}
```

**HTTP Status:** `200 OK`

---

### 2. Health Check - Readiness

**Request:**
```bash
GET /v1/readyz
```

**Response:**
```json
{
  "code": 200,
  "data": {
    "checks": [
      {},
      {}
    ],
    "ready": true
  },
  "message": "ready"
}
```

**HTTP Status:** `200 OK`

---

### 3. List Snippets (Empty)

**Request:**
```bash
GET /v1/snippets
```

**Response:**
```json
{
  "page": 1,
  "limit": 20,
  "items": []
}
```

**HTTP Status:** `200 OK`

---

### 4. Create JavaScript Snippet

**Request:**
```bash
POST /v1/snippets
Content-Type: application/json

{
  "content": "console.log('Hello, World!');",
  "tags": ["javascript", "hello-world", "console"]
}
```

**Response:**
```json
{
  "id": "7ee0902e-1d5b-435f-9559-0854c5828e37",
  "content": "console.log('Hello, World!');",
  "created_at": "2025-09-15T18:57:29Z",
  "tags": [
    "javascript",
    "hello-world",
    "console"
  ]
}
```

**HTTP Status:** `201 Created`

---

### 5. Create Python Snippet

**Request:**
```bash
POST /v1/snippets
Content-Type: application/json

{
  "content": "def hello():\n    print('Hello from Python!')",
  "tags": ["python", "function"]
}
```

**Response:**
```json
{
  "id": "8bddad48-547d-46d8-bd0c-e824eaf4d671",
  "content": "def hello():\n    print('Hello from Python!')",
  "created_at": "2025-09-15T18:57:29Z",
  "tags": [
    "python",
    "function"
  ]
}
```

**HTTP Status:** `201 Created`

---

### 6. List All Snippets

**Request:**
```bash
GET /v1/snippets
```

**Response:**
```json
{
  "page": 1,
  "limit": 20,
  "items": [
    {
      "id": "8bddad48-547d-46d8-bd0c-e824eaf4d671",
      "created_at": "2025-09-15T18:57:29Z"
    },
    {
      "id": "7ee0902e-1d5b-435f-9559-0854c5828e37",
      "created_at": "2025-09-15T18:57:29Z"
    }
  ]
}
```

**HTTP Status:** `200 OK`

**Note:** The list endpoint returns only `id` and `created_at` fields for performance. Use the individual GET endpoint to retrieve full snippet details.

---

### 7. Get Specific Snippet

**Request:**
```bash
GET /v1/snippets/8bddad48-547d-46d8-bd0c-e824eaf4d671
```

**Response:**
```json
{
  "id": "8bddad48-547d-46d8-bd0c-e824eaf4d671",
  "content": "def hello():\n    print('Hello from Python!')",
  "created_at": "2025-09-15T18:57:29Z",
  "tags": [
    "python",
    "function"
  ]
}
```

**HTTP Status:** `200 OK`

---

### 8. Update Snippet

**Request:**
```bash
PUT /v1/snippets/8bddad48-547d-46d8-bd0c-e824eaf4d671
Content-Type: application/json

{
  "content": "def hello_updated():\n    print('Hello from Updated Python!')\n    return 'success'",
  "tags": ["python", "function", "updated"]
}
```

**Response:**
```json
{
  "id": "8bddad48-547d-46d8-bd0c-e824eaf4d671",
  "content": "def hello_updated():\n    print('Hello from Updated Python!')\n    return 'success'",
  "created_at": "2025-09-15T18:57:29Z",
  "tags": [
    "python",
    "function",
    "updated"
  ]
}
```

**HTTP Status:** `200 OK`

---

### 9. Filter by Tag

**Request:**
```bash
GET /v1/snippets?tag=python
```

**Response:**
```json
{
  "page": 1,
  "limit": 20,
  "items": [
    {
      "id": "8bddad48-547d-46d8-bd0c-e824eaf4d671",
      "created_at": "2025-09-15T18:57:29Z"
    }
  ]
}
```

**HTTP Status:** `200 OK`

---

### 10. Pagination Example

**Request:**
```bash
GET /v1/snippets?page=1&limit=1
```

**Response:**
```json
{
  "page": 1,
  "limit": 1,
  "items": [
    {
      "id": "8bddad48-547d-46d8-bd0c-e824eaf4d671",
      "created_at": "2025-09-15T18:57:29Z"
    }
  ]
}
```

**HTTP Status:** `200 OK`

---

### 11. Delete Snippet

**Request:**
```bash
DELETE /v1/snippets/8bddad48-547d-46d8-bd0c-e824eaf4d671
```

**Response:**
```
(No response body)
```

**HTTP Status:** `204 No Content`

**Note:** The DELETE endpoint returns HTTP 204 (No Content) on successful deletion, following RESTful best practices.

---

### 12. Get Deleted Snippet (404)

**Request:**
```bash
GET /v1/snippets/8bddad48-547d-46d8-bd0c-e824eaf4d671
```

**Response:**
```json
{
  "error": {
    "code": "not_found",
    "message": "snippet not found"
  }
}
```

**HTTP Status:** `404 Not Found`

---

## Query Parameters

### List Snippets Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `page` | integer | 1 | Page number for pagination |
| `limit` | integer | 20 | Number of items per page (max 100) |
| `tag` | string | - | Filter snippets by tag |

### Examples:

```bash
# Get first 5 snippets
GET /v1/snippets?page=1&limit=5

# Get snippets tagged with 'javascript'
GET /v1/snippets?tag=javascript

# Get page 2 with 10 items per page, filtered by 'python' tag
GET /v1/snippets?page=2&limit=10&tag=python
```

---

## Error Responses

### 400 Bad Request
```json
{
  "error": {
    "code": "bad_request",
    "message": "invalid request",
    "details": "invalid character '!' in string escape code"
  }
}
```

### 404 Not Found
```json
{
  "error": {
    "code": "not_found",
    "message": "snippet not found"
  }
}
```

### 500 Internal Server Error
```json
{
  "error": {
    "code": "internal_error",
    "message": "internal server error"
  }
}
```

### 503 Service Unavailable
```json
{
  "code": 503,
  "data": {
    "checks": [
      {},
      {}
    ],
    "ready": false
  },
  "message": "not ready"
}
```

---

## Data Models

### Snippet Object

```json
{
  "id": "string (UUID)",
  "content": "string",
  "created_at": "string (ISO 8601 timestamp)",
  "expires_at": "string (ISO 8601 timestamp, optional)",
  "tags": ["array of strings"]
}
```

### Create/Update Snippet Request

```json
{
  "content": "string (required)",
  "tags": ["array of strings (optional)"]
}
```

### List Response

```json
{
  "page": "integer",
  "limit": "integer",
  "items": ["array of snippet objects (summary)"]
}
```

---

## Content Types

All requests and responses use `application/json` content type.

**Request Headers:**
```
Content-Type: application/json
```

**Response Headers:**
```
Content-Type: application/json; charset=utf-8
```

---

## Setup Instructions

1. **Start the services:**
   ```bash
   make setup
   ```

2. **Apply database migrations:**
   ```bash
   make migrate-apply
   ```

3. **Start the API server:**
   ```bash
   make run
   ```

4. **Test the API:**
   ```bash
   curl http://localhost:8080/v1/livez
   ```

The API will be available at `http://localhost:8080` with full CRUD operations for code snippets.