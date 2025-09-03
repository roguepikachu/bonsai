# Bonsai Snippet Service - Use Cases

## Overview

Bonsai is a **text/code snippet sharing service** that provides a fast, reliable way to store and share text content with optional expiration and tagging. Think of it as a lightweight, self-hosted alternative to Pastebin, GitHub Gists, or Hastebin.

## Core Use Cases

### 1. 📝 Code Sharing in Team Communication

**Problem:** Sharing code in Slack, Discord, or email often results in poor formatting, lost indentation, or message length limits.

**Solution:** Create a snippet and share the ID/link instead.

```bash
# Developer shares code review feedback
curl -X POST http://localhost:8080/v1/snippets \
  -H "Content-Type: application/json" \
  -d '{
    "content": "function validateEmail(email) {\n  const re = /^[^\\s@]+@[^\\s@]+\\.[^\\s@]+$/;\n  return re.test(email);\n}",
    "tags": ["javascript", "validation", "code-review"],
    "expires_in": 604800
  }'

# Response: {"id": "abc123", ...}
# Share in Slack: "Check my feedback here: http://localhost:8080/v1/snippets/abc123"
```

### 2. 🔧 Configuration File Sharing

**Problem:** Need to share environment configs, docker-compose files, or other configuration without exposing them permanently.

**Solution:** Create expiring snippets for sensitive configurations.

```json
{
  "content": "DATABASE_URL=postgres://user:pass@localhost/db\nREDIS_URL=redis://localhost:6379\nAPI_KEY=sk_test_...",
  "expires_in": 3600,  // Expires in 1 hour
  "tags": ["config", "dev-env", "temporary"]
}
```

### 3. 📋 Log Sharing for Debugging

**Problem:** Need to share error logs or stack traces with team members for debugging.

**Solution:** Store logs as snippets with relevant tags for easy retrieval.

```json
{
  "content": "[2024-01-01 12:00:00] ERROR: NullPointerException at com.example.Service.process()\n  Stack trace:\n  ...",
  "tags": ["error", "production", "java", "2024-01-01"],
  "expires_in": 2592000  // Keep for 30 days
}
```

### 4. 📚 Documentation Snippets

**Problem:** Frequently need to share the same setup instructions, API examples, or documentation snippets.

**Solution:** Create persistent snippets (no expiry) tagged for easy discovery.

```json
{
  "content": "## Local Development Setup\n\n1. Install dependencies: `npm install`\n2. Copy .env.example to .env\n3. Run migrations: `npm run migrate`\n4. Start dev server: `npm run dev`",
  "tags": ["docs", "setup", "onboarding"]
}
```

### 5. 🚀 CI/CD Pipeline Outputs

**Problem:** CI/CD logs are often too long for direct sharing, and pipeline UIs might not be accessible to all team members.

**Solution:** Store build/deployment logs as snippets.

```bash
# CI pipeline posts build output
curl -X POST http://bonsai-service/v1/snippets \
  -d '{
    "content": "Build #1234 Output:\n...",
    "tags": ["ci", "build-1234", "main-branch"],
    "expires_in": 604800
  }'
```

### 6. 💬 Interview Code Challenges

**Problem:** During technical interviews, need to share code problems or solutions temporarily.

**Solution:** Create expiring snippets for interview sessions.

```json
{
  "content": "// Problem: Implement a function to reverse a linked list\n// Time limit: 30 minutes\n\nclass ListNode {\n  constructor(val) {\n    this.val = val;\n    this.next = null;\n  }\n}",
  "expires_in": 7200,  // 2 hour interview window
  "tags": ["interview", "algorithm", "candidate-123"]
}
```

### 7. 🔐 Temporary Secret Sharing

**Problem:** Need to share API keys, passwords, or tokens securely with automatic deletion.

**Solution:** Create short-lived snippets that auto-expire.

```json
{
  "content": "Temporary AWS Access:\nAccess Key: AKIA...\nSecret: ...\nSession Token: ...",
  "expires_in": 300,  // 5 minutes only
  "tags": ["credentials", "temporary", "aws"]
}
```

### 8. 📊 SQL Query Library

**Problem:** Team needs a central place to store and share useful SQL queries.

**Solution:** Create a searchable library of SQL snippets.

```bash
# List all SQL snippets
curl "http://localhost:8080/v1/snippets?tag=sql&limit=50"

# Create new SQL snippet
curl -X POST http://localhost:8080/v1/snippets \
  -d '{
    "content": "-- Find duplicate emails\nSELECT email, COUNT(*) \nFROM users \nGROUP BY email \nHAVING COUNT(*) > 1;",
    "tags": ["sql", "postgres", "data-quality", "duplicates"]
  }'
```

### 9. 🐛 Bug Report Templates

**Problem:** Need consistent bug report format with stack traces and environment details.

**Solution:** Store bug reports as structured snippets.

```json
{
  "content": "## Bug Report\n\n**Environment:** Production\n**Service:** API Gateway\n**Time:** 2024-01-01 14:30 UTC\n\n**Description:** 500 errors on /api/users endpoint\n\n**Stack Trace:**\n```\nTypeError: Cannot read property 'id' of undefined\n  at getUserById (/app/src/handlers/user.js:45:23)\n```\n\n**Request ID:** req_abc123xyz",
  "tags": ["bug", "api", "production", "high-priority"],
  "expires_in": 2592000
}
```

### 10. 📝 Meeting Notes & Action Items

**Problem:** Need to quickly share meeting notes with attendees.

**Solution:** Create tagged snippets for meeting documentation.

```json
{
  "content": "## Sprint Planning - 2024-01-01\n\n**Attendees:** Alice, Bob, Charlie\n\n**Decisions:**\n- Prioritize auth refactor\n- Delay feature X to next sprint\n\n**Action Items:**\n- [ ] Alice: Create auth refactor ticket\n- [ ] Bob: Update sprint board\n- [ ] Charlie: Schedule demo",
  "tags": ["meeting", "sprint-planning", "team-alpha", "2024-q1"]
}
```

## Advanced Use Cases

### Tag-Based Organization

Use tags to create logical groupings:
- **By Environment:** `dev`, `staging`, `production`
- **By Team:** `team-alpha`, `team-beta`, `devops`
- **By Priority:** `urgent`, `high`, `medium`, `low`
- **By Type:** `error`, `config`, `documentation`, `script`
- **By Language:** `python`, `javascript`, `sql`, `yaml`

### Expiration Strategies

- **Immediate (5-60 min):** Passwords, tokens, sensitive data
- **Short (1-24 hours):** Temporary configs, debug logs
- **Medium (1-7 days):** Code reviews, meeting notes
- **Long (30 days):** Bug reports, incident logs
- **Permanent (no expiry):** Documentation, templates, reference code

### Integration Patterns

1. **CI/CD Integration:** Automatically post build logs
2. **Monitoring Alerts:** Store alert details as snippets
3. **Slack Bot:** Create snippets from Slack messages
4. **IDE Plugins:** Share code directly from editor
5. **CLI Tool:** Command-line snippet creation

## Benefits Over Alternatives

| Feature | Bonsai | Pastebin | GitHub Gist | Hastebin |
|---------|---------|-----------|-------------|----------|
| Self-hosted | ✅ | ❌ | ❌ | ✅ |
| Expiration | ✅ | ✅ | ❌ | ❌ |
| Tags/Categories | ✅ | ❌ | ✅ | ❌ |
| Redis Caching | ✅ | ❌ | ❌ | ❌ |
| PostgreSQL Storage | ✅ | ❌ | ❌ | ❌ |
| API-First | ✅ | Limited | ✅ | Limited |
| No Account Required | ✅ | ✅ | ❌ | ✅ |

## Security Considerations

- **No Authentication:** Snippets are accessible to anyone with the ID
- **Use Expiration:** Always set expiration for sensitive content
- **Random IDs:** IDs are randomly generated UUIDs, hard to guess
- **HTTPS Recommended:** Use HTTPS in production to prevent eavesdropping
- **Private Deployment:** Deploy internally for sensitive company data

## Best Practices

1. **Always Tag:** Use meaningful tags for easy retrieval
2. **Set Expiration:** Default to expiring content unless it's documentation
3. **Size Limits:** Keep snippets under 10KB for optimal performance
4. **Regular Cleanup:** Expired snippets are automatically cleaned up
5. **Cache Headers:** Respect X-Cache headers for performance monitoring

## Example Workflow

```bash
# 1. Developer encounters a bug
echo "Stack trace and error details..." > error.log

# 2. Create snippet
SNIPPET_ID=$(curl -s -X POST http://localhost:8080/v1/snippets \
  -H "Content-Type: application/json" \
  -d "{
    \"content\": \"$(cat error.log)\",
    \"tags\": [\"bug\", \"api\", \"urgent\"],
    \"expires_in\": 86400
  }" | jq -r '.id')

# 3. Share in team chat
echo "🐛 Found a bug, details here: http://localhost:8080/v1/snippets/$SNIPPET_ID"

# 4. Team members retrieve and debug
curl http://localhost:8080/v1/snippets/$SNIPPET_ID | jq '.content'

# 5. List all urgent bugs
curl "http://localhost:8080/v1/snippets?tag=urgent"
```

## Conclusion

Bonsai is ideal for teams that need a simple, fast, and reliable way to share text content temporarily or permanently. It's particularly useful for:

- Development teams sharing code and configs
- DevOps teams sharing logs and scripts
- Support teams sharing troubleshooting steps
- Any scenario requiring temporary text storage with automatic cleanup

The combination of PostgreSQL persistence, Redis caching, and tag-based organization makes it a powerful tool for managing shared text content in a team environment.