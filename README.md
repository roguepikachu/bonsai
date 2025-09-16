# Bonsai API

A modern RESTful API for managing code snippets with PostgreSQL backend, Redis caching, and comprehensive testing automation.

## 🚀 Quick Start

```bash
# Complete setup, migration, and testing
make setup-and-test

# Basic development workflow
make setup          # First time setup
make run            # Start the API server
make test           # Run tests
make clean          # Clean up everything
```

## 📖 Table of Contents

- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Development Setup](#development-setup)
- [API Documentation](#api-documentation)
- [Database Management](#database-management)
- [Testing](#testing)
- [Deployment](#deployment)
- [Project Structure](#project-structure)
- [Contributing](#contributing)

## 🏗 Architecture

### Tech Stack
- **Language**: Go 1.23+ (minimum 1.21)
- **Web Framework**: Gin
- **Database**: PostgreSQL 16
- **Cache**: Redis 7
- **Migrations**: Atlas CLI
- **Containerization**: Docker & Docker Compose
- **CI/CD**: GitHub Actions

### System Components
- **API Server**: RESTful service with CRUD operations for code snippets
- **Database Layer**: PostgreSQL with JSONB support for tags and optimized indexing
- **Caching Layer**: Redis for session management and performance optimization
- **Migration System**: Atlas-powered declarative schema management
- **Testing Suite**: Comprehensive unit and integration tests with automated API sanity checks

## 🔧 Prerequisites

- **Go**: 1.23+ (minimum 1.21)
- **Docker**: Latest version with Docker Compose
- **Atlas CLI**: Installed automatically via `make migrate-install`

### Optional Tools
- **curl**: For API testing
- **jq**: For JSON response formatting
- **PostgreSQL client tools**: For direct database access

## 🛠 Development Setup

### 1. Clone and Initial Setup
```bash
git clone <repository-url>
cd bonsai
make setup
```

This will:
- Create `.env` file from `.env.example` if needed
- Start PostgreSQL and Redis containers
- Install Go dependencies

### 2. Database Migration
```bash
make migrate-apply
```

### 3. Start the API Server

#### Option A: Development mode (includes migrations)
```bash
make dev
```

#### Option B: Run server only
```bash
make run
```

The API will be available at `http://localhost:8080`

### 4. Verify Installation
```bash
# Quick health check
curl http://localhost:8080/v1/livez

# Comprehensive API testing
make sanity-check
```

## 📚 API Documentation

### Base URL
```
http://localhost:8080
```

### Health Endpoints
- `GET /v1/livez` - Liveness check
- `GET /v1/readyz` - Readiness check with database connectivity

### Snippet Operations
- `POST /v1/snippets` - Create a new snippet
- `GET /v1/snippets` - List snippets (with pagination and filtering)
- `GET /v1/snippets/{id}` - Get specific snippet
- `PUT /v1/snippets/{id}` - Update snippet
- `DELETE /v1/snippets/{id}` - Delete snippet

### Query Parameters
| Parameter | Type | Description | Example |
|-----------|------|-------------|---------|
| `page` | integer | Page number (default: 1) | `?page=2` |
| `limit` | integer | Items per page (default: 20, max: 100) | `?limit=10` |
| `tag` | string | Filter by tag | `?tag=javascript` |

### Example Usage
```bash
# Create a snippet
curl -X POST http://localhost:8080/v1/snippets \
  -H "Content-Type: application/json" \
  -d '{"content": "console.log(\"Hello World\");", "tags": ["javascript", "hello-world"]}'

# List snippets with filtering
curl "http://localhost:8080/v1/snippets?tag=javascript&limit=5"

# Get specific snippet
curl http://localhost:8080/v1/snippets/{snippet-id}
```

**📋 For complete API examples with request/response samples, see [API_EXAMPLES.md](./API_EXAMPLES.md)**

## 🗄 Database Management

### Migration System
This project uses **Atlas** for declarative database schema management.

#### Key Files
- `atlas.hcl` - Atlas configuration
- `schema/snippets.hcl` - Declarative database schema
- `atlas-migrations/` - Generated migration files

#### Common Commands
```bash
# Check migration status
make migrate-status

# Apply pending migrations
make migrate-apply

# Generate new migration (after schema changes)
DATABASE_URL="postgres://..." atlas migrate diff --env local
```

#### Schema Structure
```sql
-- Snippets table with optimized indexing
CREATE TABLE snippets (
  id          TEXT PRIMARY KEY,
  content     TEXT NOT NULL,
  tags        JSONB NOT NULL DEFAULT '[]',
  created_at  TIMESTAMPTZ NOT NULL,
  expires_at  TIMESTAMPTZ NULL
);

-- Performance indexes
CREATE INDEX idx_snippets_created_at ON snippets (created_at DESC);
CREATE INDEX idx_snippets_expires_at ON snippets (expires_at);
CREATE INDEX idx_snippets_tags_gin ON snippets USING gin (tags);
```

### Database Environments
- **Development**: `bonsai` database
- **Testing**: `bonsai_test` database
- **Docker**: Both databases auto-created via init script

## 🧪 Testing

### Test Categories
1. **Unit Tests**: Fast, isolated component tests
2. **Integration Tests**: Database and Redis integration
3. **API Sanity Tests**: Comprehensive end-to-end API validation
4. **Simple API Tests**: Quick functionality verification

### Running Tests
```bash
# Unit tests only
make test-unit

# Integration tests (requires services)
make test-integration

# All tests
make test-all

# API sanity check
make sanity-check

# Complete setup + testing pipeline
make setup-and-test

# Simple API test (quick verification)
make test-simple
```

### API Sanity Test Suite
The automated test suite (`scripts/api-sanity-check.sh`) validates:

1. ✅ Health endpoints (liveness/readiness)
2. ✅ Snippet CRUD operations
3. ✅ Pagination and filtering
4. ✅ Error handling
5. ✅ Data persistence
6. ✅ Edge cases and validation

**Test Results Example:**
```bash
================================
[INFO] Test Summary
Total Tests: 12
Passed: 12
Failed: 0
✅ All tests passed! API is healthy.
```

### Important Notes
- The DELETE endpoint returns HTTP 204 (No Content) on success
- Readiness check validates both PostgreSQL and Redis connections
- All tests use Atlas migrations (no AUTO_MIGRATE)

## 🚢 Deployment

### Docker Compose
```bash
# Start all services
make services

# Stop all services
make services-stop

# View logs
make logs
```

### Kubernetes Deployment
Kubernetes manifests are available in the `k8s/` directory:

- `atlas-configmap.yaml` - Migration configuration
- Database and Redis deployment configs

```bash
# Apply Kubernetes configs
kubectl apply -f k8s/
```

### Environment Variables
| Variable | Description | Default |
|----------|-------------|---------|
| `POSTGRES_URL` | PostgreSQL connection string (DSN format) | Required - no default |
| `DATABASE_URL` | Used by Atlas CLI for migrations only | Required for migrations |
| `REDIS_URL` | Redis connection string | `redis://localhost:6379` |
| `BONSAI_PORT` | API server port | `8080` |
| `GIN_MODE` | Gin framework mode | `debug` |

**Note**: The application uses `POSTGRES_URL` for database connections, while `DATABASE_URL` is only used by Atlas CLI for schema migrations.

## 📁 Project Structure

```
bonsai/
├── cmd/
│   └── api/                    # Application entry point
├── internal/
│   ├── http/                   # HTTP handlers and middleware
│   ├── service/               # Business logic layer
│   └── repository/            # Data access layer
├── atlas-migrations/          # Database migration files
├── schema/                    # Atlas schema definitions
├── scripts/                   # Automation scripts
│   └── api-sanity-check.sh   # Comprehensive API testing
├── docker/                    # Docker configuration
├── k8s/                       # Kubernetes manifests
├── API_EXAMPLES.md            # Complete API documentation with examples
├── atlas.hcl                  # Atlas configuration
├── Makefile                   # Development automation
└── README.md                  # This file
```

### Key Components

#### Core Application (`internal/`)
- **HTTP Layer**: Gin-based REST API with middleware
- **Service Layer**: Business logic and validation
- **Repository Layer**: Database abstractions and queries

#### Database Management
- **Atlas Schema**: Declarative schema in `schema/snippets.hcl`
- **Migrations**: Auto-generated in `atlas-migrations/`
- **Configuration**: Multi-environment setup in `atlas.hcl`

#### Automation & Testing
- **Makefile**: 14 essential development commands
- **Sanity Tests**: Comprehensive API validation script
- **CI/CD**: GitHub Actions with drift detection

## 🤝 Contributing

### Development Workflow
1. **Setup**: `make setup`
2. **Develop**: Make your changes
3. **Test**: `make test-all`
4. **Validate**: `make sanity-check`
5. **Clean**: `make clean`

### Code Standards
- Follow Go best practices and idioms
- Maintain test coverage for new features
- Update API examples when modifying endpoints
- Use declarative schema changes via Atlas

### Schema Changes
1. Modify `schema/snippets.hcl`
2. Generate migration: `atlas migrate diff --env local`
3. Test migration: `make migrate-apply`
4. Commit both schema and migration files

## 📊 Make Targets Reference

| Target | Description |
|--------|-------------|
| `setup` | First-time development environment setup |
| `run` | Start the API server |
| `services` | Start PostgreSQL and Redis containers |
| `services-stop` | Stop all services |
| `migrate-apply` | Apply database migrations |
| `migrate-status` | Check migration status |
| `test-unit` | Run unit tests |
| `test-integration` | Run integration tests |
| `test-all` | Run all tests |
| `sanity-check` | Run comprehensive API tests |
| `test-simple` | Run quick API functionality test |
| `dev` | Start development environment with migrations |
| `setup-and-test` | Complete automation pipeline |
| `build` | Build the binary |
| `clean` | Clean up everything |
| `help` | Show all available targets |

## 🔍 Troubleshooting

### Common Issues

**Database Connection Issues**
```bash
# Check if PostgreSQL is running
docker ps | grep postgres

# Verify database connectivity
make migrate-status
```

**Port Conflicts**
```bash
# Check what's using port 8080
lsof -ti:8080

# Use different port
BONSAI_PORT=8081 make run
```

**Migration Failures**
```bash
# Reset test database
make test-cleanup

# Check Atlas configuration
atlas migrate status --env local
```

### Getting Help
- Check the logs: `make logs`
- Run health checks: `curl http://localhost:8080/v1/livez`
- Validate setup: `make sanity-check`

## 📈 Performance & Monitoring

### Database Optimizations
- **GIN Index**: Fast tag-based filtering with JSONB
- **B-tree Indexes**: Optimized sorting by creation date
- **Connection Pooling**: Configured PostgreSQL connection management

### Caching Strategy
- **Redis Integration**: Session management and caching layer
- **Concurrent Operations**: Safe for high-throughput scenarios

### Monitoring Endpoints
- `/v1/livez` - Basic health check
- `/v1/readyz` - Database and Redis connectivity verification

## 📜 License

This project is licensed under the [MIT License](LICENSE).

---

**🌟 Happy coding with Bonsai!**

For questions or support, please check the troubleshooting section or open an issue.