# Quick Start Guide

## What is Bonsai?

Bonsai is a text snippet sharing service - like Pastebin or GitHub Gists. It's designed for sharing:
- Code snippets
- Configuration files
- Log outputs
- Any text content that needs temporary or permanent storage

## Basic Usage

### 1. Create a Snippet

```bash
# Share some code
curl -X POST http://localhost:8080/v1/snippets \
  -H "Content-Type: application/json" \
  -d '{
    "content": "console.log(\"Hello, World!\");",
    "tags": ["javascript", "example"],
    "expires_in": 3600
  }'

# Response
{
  "id": "abc123",
  "created_at": "2024-01-01T12:00:00Z",
  "expires_at": "2024-01-01T13:00:00Z",
  "tags": ["javascript", "example"]
}
```

### 2. Retrieve a Snippet

```bash
curl http://localhost:8080/v1/snippets/abc123

# Response
{
  "id": "abc123",
  "content": "console.log(\"Hello, World!\");",
  "created_at": "2024-01-01T12:00:00Z",
  "expires_at": "2024-01-01T13:00:00Z",
  "tags": ["javascript", "example"]
}
```

### 3. List Snippets by Tag

```bash
curl "http://localhost:8080/v1/snippets?tag=javascript&limit=10"

# Response
{
  "page": 1,
  "limit": 10,
  "items": [
    {
      "id": "abc123",
      "created_at": "2024-01-01T12:00:00Z",
      "expires_at": "2024-01-01T13:00:00Z"
    }
  ]
}
```

## Common Use Cases

### Share Code in Slack/Discord

```bash
# Create snippet
SNIPPET_ID=$(curl -s -X POST http://localhost:8080/v1/snippets \
  -d '{"content": "YOUR_CODE_HERE", "tags": ["review"]}' | jq -r '.id')

# Share in chat
echo "Check my code: http://localhost:8080/v1/snippets/$SNIPPET_ID"
```

### Temporary Config Sharing

```bash
# Share config that auto-deletes in 1 hour
curl -X POST http://localhost:8080/v1/snippets \
  -d '{
    "content": "DATABASE_URL=postgres://...",
    "expires_in": 3600,
    "tags": ["config", "temp"]
  }'
```

### Store Debug Logs

```bash
# Save error log
cat error.log | curl -X POST http://localhost:8080/v1/snippets \
  -d @- \
  -H "Content-Type: application/json" \
  --data-raw '{
    "content": "'"$(cat error.log)"'",
    "tags": ["error", "production"],
    "expires_in": 2592000
  }'
```

## Expiration Times

- `300` - 5 minutes (passwords, tokens)
- `3600` - 1 hour (temp configs)
- `86400` - 1 day (code reviews)
- `604800` - 1 week (meeting notes)
- `2592000` - 30 days (max, for logs)
- `0` or omit - Never expires (documentation)

## Tags Best Practices

Use tags to organize snippets:
- By language: `python`, `javascript`, `sql`
- By purpose: `config`, `error`, `documentation`
- By environment: `dev`, `staging`, `production`
- By team: `frontend`, `backend`, `devops`

## Performance Tips

- Snippets are cached in Redis for fast retrieval
- The `X-Cache` header tells you if content was served from cache
- Keep snippets under 10KB for optimal performance
- Use expiration to automatically clean up old content