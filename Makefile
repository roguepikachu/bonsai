##@ Globals
DOCKER_COMPOSE="docker/docker-compose.yaml"
DOTENV_PATHS?=.env

# Detect docker compose CLI (docker compose vs docker-compose)
ifeq (, $(shell command -v docker-compose 2>/dev/null))
DC=docker compose -f $(DOCKER_COMPOSE)
else
DC=docker-compose -f $(DOCKER_COMPOSE)
endif

GO?=go
PKG?=./...
GOTESTFLAGS?=
IMAGE?=bonsai
TAG?=latest

.DEFAULT_GOAL := help

##@ Meta
.PHONY: help
help: ## Show help
	@echo "Usage:\n  make \033[36m<target>\033[0m\n"; \
	awk 'BEGIN {FS = ":.*##"} \
	/^##@/ { print "\n\033[1m" substr($$0,5) "\033[0m" } \
	/^[a-zA-Z0-9_.-]+:.*##/ { printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

##@ Quality
.PHONY: lint fmt tidy
lint: fmt ## Run golangci-lint
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not found. Install via: brew install golangci-lint"; exit 1; }
	golangci-lint run

fmt: ## Format code
	$(GO) fmt $(PKG)

tidy: ## Tidy go.mod and go.sum
	$(GO) mod tidy

##@ Tests
.PHONY: test test-unit test-integration test-e2e test-all
test: test-unit ## Run unit tests (default test target)

test-unit: ## Run unit tests with coverage (no external services required)
	$(GO) test -race -coverprofile=coverage.out -covermode=atomic $(GOTESTFLAGS) -short $(PKG)
	$(GO) tool cover -func=coverage.out

test-integration: ## Run integration tests with coverage (requires services up)
	$(GO) test -tags=integration -race -coverprofile=coverage-integration.out -covermode=atomic $(GOTESTFLAGS) $(PKG)

test-e2e: ## Run end-to-end acceptance tests (starts and stops services automatically)
	$(GO) test -race -v $(GOTESTFLAGS) ./internal/http/acceptance -run "TestE2E"

test-all: test-unit test-integration test-e2e ## Run all tests

##@ Dev services (Docker)
.PHONY: db-up db-down redis-up redis-down redis-logs redis-restart postgres-up postgres-down postgres-logs postgres-restart dev-up dev-down

db-up: ## Start Redis and Postgres containers
	$(DC) up -d redis postgres

db-down: ## Stop and remove dev dependencies (Redis + Postgres)
	$(DC) down

redis-up: ## Start Redis only
	$(DC) up -d redis

redis-down: ## Stop Redis container
	$(DC) stop redis

redis-logs: ## Tail Redis logs
	$(DC) logs -f redis

redis-restart: redis-down redis-up ## Restart Redis container

postgres-up: ## Start Postgres only
	$(DC) up -d postgres

postgres-down: ## Stop Postgres container
	$(DC) stop postgres

postgres-logs: ## Tail Postgres logs
	$(DC) logs -f postgres

postgres-restart: postgres-down postgres-up ## Restart Postgres container

dev-up: ## Start all dev dependencies (Redis + Postgres)
	@$(MAKE) db-up

dev-down: ## Stop and remove dev dependencies (Redis + Postgres)
	@$(MAKE) db-down



##@ Workflow
.PHONY: bootstrap dev up down
bootstrap: ## Initialize local env and start dependencies
	@[ -f .env ] || { [ -f .env.example ] && cp -n .env.example .env || true; }
	$(MAKE) dev-up
	$(MAKE) tidy

dev: bonsai-run ## Run the API locally from source

up: dev-up ## Start DBs and run the API together
	DOTENV_PATHS=$(DOTENV_PATHS) $(GO) run ./cmd/api/main.go

down: dev-down ## Stop DBs

##@ Build & Run
.PHONY: bonsai-build
bonsai-build: ## Build the bonsai binary
	$(GO) build -o bonsai ./cmd/api/main.go

.PHONY: bonsai-run
bonsai-run: ## Run the bonsai server (from source)
	DOTENV_PATHS=$(DOTENV_PATHS) $(GO) run ./cmd/api/main.go

.PHONY: bonsai-image image docker-image docker-build
bonsai-image: ## Build Docker image (override with IMAGE and TAG)
	docker build -t $(IMAGE):$(TAG) .

# Aliases
image: bonsai-image ## Alias for bonsai-image
docker-image: bonsai-image ## Alias for bonsai-image
docker-build: bonsai-image ## Alias for bonsai-image

.PHONY: start
start: bonsai-build ## Run bonsai binary (after build)
	DOTENV_PATHS=$(DOTENV_PATHS) ./bonsai

.PHONY: probes
probes: ## Hit local health endpoints
	curl -sS http://localhost:8080/v1/health | jq . || true
	curl -sS http://localhost:8080/v1/livez | jq . || true
	curl -sS http://localhost:8080/v1/readyz | jq . || true