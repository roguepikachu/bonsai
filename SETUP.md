# Bonsai API - Complete Setup & Development Guide

## 📋 Table of Contents
- [Architecture Overview](#architecture-overview)
- [Environment Setup](#environment-setup)
- [Database Configuration](#database-configuration)
- [Running the Application](#running-the-application)
- [Testing Strategy](#testing-strategy)
- [Deployment](#deployment)
- [Troubleshooting](#troubleshooting)

## 🏗 Architecture Overview

### System Components

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│                 │     │                 │     │                 │
│   Gin Router    │────▶│  Service Layer  │────▶│   Repository    │
│   (HTTP API)    │     │  (Business)     │     │   (Database)    │
│                 │     │                 │     │                 │
└─────────────────┘     └─────────────────┘     └─────────────────┘
         │                       │                        │
         │                       │                        │
         ▼                       ▼                        ▼
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│                 │     │                 │     │                 │
│  Health Checks  │     │  Redis Cache    │     │  PostgreSQL DB  │
│   /v1/livez     │     │  (Session Mgmt) │     │  (Data Store)   │
│   /v1/readyz    │     │                 │     │                 │
└─────────────────┘     └─────────────────┘     └─────────────────┘
```

### Key Technologies
- **Go 1.23+ (minimum 1.21)**: Core application language
- **Gin Web Framework**: HTTP routing and middleware
- **PostgreSQL 16**: Primary data storage with JSONB support
- **Redis 7**: Caching and session management
- **Atlas CLI**: Database schema management and migrations
- **Docker & Docker Compose**: Containerization for services

## 🔧 Environment Setup

### Prerequisites Installation

#### macOS
```bash
# Install Homebrew (if not installed)
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# Install Go
brew install go

# Install Docker Desktop
brew install --cask docker

# Install Atlas CLI
curl -sSf https://atlasgo.sh | sh

# Install PostgreSQL client tools (optional)
brew install postgresql

# Install Redis client (optional)
brew install redis
```

#### Linux (Ubuntu/Debian)
```bash
# Install Go
wget https://go.dev/dl/go1.23.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.23.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Install Docker
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# Install Atlas CLI
curl -sSf https://atlasgo.sh | sh

# Install PostgreSQL client
sudo apt-get install postgresql-client

# Install Redis client
sudo apt-get install redis-tools
```

### Environment Variables

The application uses two different environment variable configurations:

| Variable | Used By | Purpose | Example |
|----------|---------|---------|---------|
| `POSTGRES_URL` | **Application** | Database connection for the API | `postgres://user:pass@localhost:5432/bonsai?sslmode=disable` |
| `DATABASE_URL` | **Atlas CLI** | Database connection for migrations | Same format as POSTGRES_URL |
| `REDIS_URL` | Application | Redis connection | `redis://localhost:6379` |
| `BONSAI_PORT` | Application | API server port | `8080` |
| `GIN_MODE` | Application | Gin framework mode | `debug` or `release` |

**Important**: The application reads `POSTGRES_URL` while Atlas CLI uses `DATABASE_URL`. Both must point to the same database.

### Project Structure
```
bonsai/
├── cmd/api/              # Application entry point
├── internal/             # Core application code
│   ├── config/          # Environment configuration
│   ├── http/            # HTTP handlers and middleware
│   ├── repository/      # Database access layer
│   └── service/         # Business logic layer
├── schema/              # Atlas schema definitions (HCL)
├── atlas-migrations/    # Generated migration files
├── scripts/             # Utility scripts
│   ├── api-sanity-check.sh    # Comprehensive API tests
│   └── simple-api-test.sh     # Quick API tests
├── docker/              # Docker configurations
├── k8s/                 # Kubernetes manifests
├── .env.example         # Environment template
├── atlas.hcl            # Atlas configuration
├── Makefile             # Development automation
└── go.mod               # Go module definition
```

## 💾 Database Configuration

### PostgreSQL Setup

The application uses PostgreSQL with the following schema:

```sql
CREATE TABLE snippets (
    id TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ
);

-- Performance indexes
CREATE INDEX idx_snippets_created_at ON snippets (created_at DESC);
CREATE INDEX idx_snippets_expires_at ON snippets (expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX idx_snippets_tags_gin ON snippets USING gin (tags);
```

### Atlas Migration System

Atlas provides declarative schema management:

1. **Schema Definition** (`schema/snippets.hcl`):
```hcl
table "snippets" {
  schema = schema.public
  column "id" {
    type = text
    null = false
  }
  column "content" {
    type = text
    null = false
  }
  column "tags" {
    type = jsonb
    null = false
    default = sql("'[]'::jsonb")
  }
  primary_key {
    columns = [column.id]
  }
}
```

2. **Migration Commands**:
```bash
# Check migration status
make migrate-status

# Apply migrations
make migrate-apply

# Generate new migration after schema changes
DATABASE_URL="postgres://..." atlas migrate diff --env local
```

## 🚀 Running the Application

### Quick Start (Recommended)
```bash
# Complete setup with migrations
make dev
```

This command:
1. Starts PostgreSQL and Redis containers
2. Waits for database readiness
3. Applies Atlas migrations
4. Starts the API server

### Manual Step-by-Step

#### 1. Start Services
```bash
# Start PostgreSQL and Redis
make services

# Or using docker-compose directly
docker compose -f docker/docker-compose.yaml up -d
```

#### 2. Apply Migrations
```bash
# Set DATABASE_URL for Atlas
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/bonsai?sslmode=disable"

# Apply migrations
make migrate-apply
```

#### 3. Start API Server
```bash
# Set environment variables
export POSTGRES_URL="postgres://postgres:postgres@localhost:5432/bonsai?sslmode=disable"
export REDIS_URL="redis://localhost:6379"
export BONSAI_PORT=8080

# Run the server
go run ./cmd/api/main.go

# Or use make
make run
```

### Verify Installation

#### Health Checks
```bash
# Liveness check (basic health)
curl http://localhost:8080/v1/livez
# Response: {"code":200,"data":{},"message":"ok"}

# Readiness check (validates DB and Redis)
curl http://localhost:8080/v1/readyz
# Response: {"code":200,"data":{"checks":[{},{}],"ready":true},"message":"ready"}
```

#### Quick Functionality Test
```bash
# Run simple API tests
make test-simple

# Or comprehensive sanity check
make sanity-check
```

## 🧪 Testing Strategy

### Test Levels

#### 1. Unit Tests
Fast, isolated tests without external dependencies:
```bash
make test-unit
```

#### 2. Integration Tests
Tests with real database and Redis:
```bash
make test-integration
```

#### 3. API Tests

**Simple Test** (quick verification):
```bash
make test-simple
# Tests: Health, CRUD operations
# Duration: ~5 seconds
```

**Comprehensive Sanity Check**:
```bash
make sanity-check
# Tests: All endpoints, pagination, filtering, error cases
# Duration: ~30 seconds
```

### Running All Tests
```bash
# Complete test suite
make test-all

# Setup + all tests
make setup-and-test
```

### Test Coverage
```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
open coverage.html  # macOS
```

## 🚢 Deployment

### Docker Build
```bash
# Build Docker image
docker build -t bonsai:latest .

# Run containerized
docker run -p 8080:8080 \
  -e POSTGRES_URL="postgres://..." \
  -e REDIS_URL="redis://..." \
  bonsai:latest
```

### Kubernetes Deployment

1. **Create Secrets**:
```bash
# Database secret
kubectl create secret generic bonsai-database \
  --from-literal=url="postgres://user:pass@host:5432/bonsai?sslmode=require"

# Redis secret
kubectl create secret generic bonsai-redis \
  --from-literal=url="redis://redis-service:6379"
```

2. **Apply Manifests**:
```bash
# Apply all K8s resources
kubectl apply -f k8s/

# Check deployment
kubectl get pods -l app=bonsai
kubectl logs -l app=bonsai
```

### Production Considerations

#### Environment Variables
```bash
# Production settings
export GIN_MODE=release
export POSTGRES_URL="postgres://prod_user:secure_pass@db.example.com:5432/bonsai?sslmode=require"
export REDIS_URL="redis://:auth_pass@redis.example.com:6379/0"
```

#### Health Check Configuration
- **Liveness**: `/v1/livez` - Basic application health
- **Readiness**: `/v1/readyz` - Database and Redis connectivity
- Configure appropriate timeouts and thresholds in deployment

## 🔍 Troubleshooting

### Common Issues

#### 1. Database Connection Failed
```bash
# Error: "password authentication failed for user postgres"
# Solution: Ensure POSTGRES_URL is set (not DATABASE_URL for the app)
export POSTGRES_URL="postgres://postgres:postgres@localhost:5432/bonsai?sslmode=disable"
```

#### 2. Readiness Check Returns 503
```bash
# Check services are running
docker ps | grep -E "postgres|redis"

# Test database connection
docker exec postgres pg_isready -U postgres

# Test Redis connection
redis-cli ping
```

#### 3. Migration Errors
```bash
# "relation snippets already exists"
# Solution: Check migration status
DATABASE_URL="..." atlas migrate status --env local

# Force reapply (development only)
DATABASE_URL="..." atlas migrate apply --env local --allow-dirty
```

#### 4. Port Already in Use
```bash
# Find process using port 8080
lsof -ti:8080

# Kill process
kill -9 $(lsof -ti:8080)

# Or use different port
BONSAI_PORT=8081 make run
```

#### 5. Delete Returns 204 Instead of 200
This is correct behavior. RESTful APIs should return:
- **204 No Content** for successful DELETE operations
- **200 OK** when returning a response body

The test scripts have been updated to accept both.

### Debug Commands

#### View Logs
```bash
# Docker logs
docker logs postgres
docker logs redis

# Application logs (if running via make)
make dev 2>&1 | tee app.log
```

#### Database Access
```bash
# Connect to PostgreSQL
docker exec -it postgres psql -U postgres -d bonsai

# Common queries
SELECT COUNT(*) FROM snippets;
SELECT * FROM snippets ORDER BY created_at DESC LIMIT 5;
```

#### Redis Access
```bash
# Connect to Redis
docker exec -it redis redis-cli

# Common commands
KEYS *
FLUSHALL  # Clear all data (careful!)
```

### Clean Slate Reset
```bash
# Complete cleanup and restart
make clean          # Stop services, clean artifacts
make services-stop  # Ensure services are stopped
docker volume rm docker_pg-data docker_redis-data  # Remove data volumes
make dev           # Fresh start
```

## 📚 Additional Resources

- [API Examples](./API_EXAMPLES.md) - Complete API usage examples
- [Atlas Documentation](https://atlasgo.io/docs) - Schema management
- [Gin Framework](https://gin-gonic.com/docs/) - Web framework documentation
- [Go Best Practices](https://go.dev/doc/effective_go) - Language guidelines

## 🤝 Development Workflow

1. **Make changes to schema** (`schema/snippets.hcl`)
2. **Generate migration**:
   ```bash
   DATABASE_URL="..." atlas migrate diff --env local
   ```
3. **Apply migration**:
   ```bash
   make migrate-apply
   ```
4. **Run tests**:
   ```bash
   make test-all
   ```
5. **Verify with sanity check**:
   ```bash
   make sanity-check
   ```

## 📝 Important Notes

- **NO AUTO_MIGRATE**: The application uses only Atlas for migrations
- **Environment Variables**: `POSTGRES_URL` for app, `DATABASE_URL` for Atlas
- **Health Checks**: Readiness validates both PostgreSQL and Redis
- **HTTP Status Codes**: DELETE returns 204, not 200
- **Testing**: All tests use real databases, no mocks

---

For issues or questions, check the [troubleshooting section](#troubleshooting) or open an issue on GitHub.