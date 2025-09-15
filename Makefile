# Bonsai API Makefile
# Simple commands to develop, test, and deploy the Bonsai snippet service

# Variables
GO := go
PKG := ./...
BINARY := bonsai
DOCKER_COMPOSE := docker/docker-compose.yaml

# Detect docker compose command
DOCKER_COMPOSE_CMD := $(shell docker compose version > /dev/null 2>&1 && echo "docker compose" || echo "docker-compose")

.DEFAULT_GOAL := help

# Colors for better output
COLOR_GREEN := \033[0;32m
COLOR_BLUE := \033[0;34m
COLOR_YELLOW := \033[0;33m
COLOR_RESET := \033[0m

##@ Quick Start
.PHONY: setup run
setup: ## First time setup - install deps and start services
	@echo "$(COLOR_GREEN)Setting up Bonsai development environment...$(COLOR_RESET)"
	@[ -f .env ] || { [ -f .env.example ] && cp .env.example .env || echo "Created .env file"; }
	$(MAKE) services
	$(GO) mod tidy
	@echo "$(COLOR_GREEN)Setup complete! Run 'make run' to start the server$(COLOR_RESET)"

run: services ## Start the API server (with database services)
	@echo "$(COLOR_BLUE)Starting Bonsai API server...$(COLOR_RESET)"
	$(GO) run ./cmd/api/main.go

##@ Development
.PHONY: dev build clean install
dev: ## Run in development mode (live reload)
	@echo "$(COLOR_BLUE)Running in development mode...$(COLOR_RESET)"
	$(MAKE) services
	$(GO) run ./cmd/api/main.go

build: ## Build the binary
	@echo "$(COLOR_BLUE)Building $(BINARY)...$(COLOR_RESET)"
	$(GO) build -o $(BINARY) ./cmd/api/main.go

install: build ## Install binary to $GOPATH/bin
	$(GO) install ./cmd/api

clean: ## Clean up built artifacts, test resources, and stop services
	@echo "$(COLOR_YELLOW)Cleaning up...$(COLOR_RESET)"
	rm -f $(BINARY)
	rm -f coverage*.out coverage*.html
	$(GO) clean
	@$(MAKE) test-cleanup
	@$(MAKE) services-stop

##@ Database Migrations (Legacy golang-migrate)
.PHONY: migrate-up migrate-down migrate-reset migrate-version migrate-force
migrate-up: ## Run database migrations up (legacy)
	@echo "$(COLOR_BLUE)Running database migrations...$(COLOR_RESET)"
	$(GO) run ./cmd/migrate/main.go -database-url "$(DATABASE_URL)" -path ./migrations up

migrate-down: ## Rollback all database migrations (legacy)
	@echo "$(COLOR_YELLOW)Rolling back database migrations...$(COLOR_RESET)"
	$(GO) run ./cmd/migrate/main.go -database-url "$(DATABASE_URL)" -path ./migrations down

migrate-reset: ## Reset database (down + up) (legacy)
	@echo "$(COLOR_YELLOW)Resetting database...$(COLOR_RESET)"
	@$(MAKE) migrate-down
	@$(MAKE) migrate-up

migrate-version: ## Show current migration version (legacy)
	$(GO) run ./cmd/migrate/main.go -database-url "$(DATABASE_URL)" -path ./migrations version

migrate-force: ## Force migration version (usage: make migrate-force VERSION=1) (legacy)
	@if [ -z "$(VERSION)" ]; then echo "$(COLOR_YELLOW)Usage: make migrate-force VERSION=<version>$(COLOR_RESET)"; exit 1; fi
	$(GO) run ./cmd/migrate/main.go -database-url "$(DATABASE_URL)" -path ./migrations force $(VERSION)

##@ Atlas Schema Migrations
.PHONY: atlas-install atlas-diff atlas-apply atlas-status atlas-lint atlas-validate atlas-dry-run atlas-baseline
# Check if ATLAS_ENV is set, default to local
ATLAS_ENV ?= local

atlas-install: ## Install Atlas CLI
	@echo "$(COLOR_BLUE)Installing Atlas CLI...$(COLOR_RESET)"
	@command -v atlas >/dev/null 2>&1 || { \
		echo "$(COLOR_YELLOW)Atlas not found, installing...$(COLOR_RESET)"; \
		curl -sSf https://atlasgo.sh | sh; \
	}
	@echo "$(COLOR_GREEN)Atlas CLI installed successfully!$(COLOR_RESET)"

atlas-diff: atlas-install ## Generate new migration from schema changes
	@echo "$(COLOR_BLUE)Generating Atlas migration diff...$(COLOR_RESET)"
	@if [ -z "$(NAME)" ]; then echo "$(COLOR_YELLOW)Usage: make atlas-diff NAME=<migration_name>$(COLOR_RESET)"; exit 1; fi
	atlas migrate diff $(NAME) --env $(ATLAS_ENV)
	@echo "$(COLOR_GREEN)Migration diff generated!$(COLOR_RESET)"

atlas-apply: atlas-install ## Apply pending migrations to database
	@echo "$(COLOR_BLUE)Applying Atlas migrations...$(COLOR_RESET)"
	atlas migrate apply --env $(ATLAS_ENV)
	@echo "$(COLOR_GREEN)Migrations applied successfully!$(COLOR_RESET)"

atlas-status: atlas-install ## Show migration status
	@echo "$(COLOR_BLUE)Checking Atlas migration status...$(COLOR_RESET)"
	atlas migrate status --env $(ATLAS_ENV)

atlas-lint: atlas-install ## Lint migration files
	@echo "$(COLOR_BLUE)Linting Atlas migrations...$(COLOR_RESET)"
	atlas migrate lint --env $(ATLAS_ENV) --latest 1

atlas-validate: atlas-install ## Validate schema definition
	@echo "$(COLOR_BLUE)Validating Atlas schema...$(COLOR_RESET)"
	atlas schema validate --env $(ATLAS_ENV)

atlas-dry-run: atlas-install ## Show migration plan without applying (dry run)
	@echo "$(COLOR_BLUE)Atlas migration dry run...$(COLOR_RESET)"
	atlas migrate apply --env $(ATLAS_ENV) --dry-run

atlas-baseline: atlas-install ## Mark current database state as baseline
	@echo "$(COLOR_BLUE)Setting Atlas baseline...$(COLOR_RESET)"
	atlas migrate set --env $(ATLAS_ENV) 20250915100000

atlas-drift: atlas-install ## Detect schema drift between desired and live database
	@echo "$(COLOR_BLUE)Checking for schema drift...$(COLOR_RESET)"
	@atlas schema diff --env $(ATLAS_ENV) --from "$(DATABASE_URL)" --to file://schema && \
		echo "$(COLOR_GREEN)No drift detected$(COLOR_RESET)" || \
		{ echo "$(COLOR_YELLOW)Schema drift detected! Run 'make atlas-diff' to generate migrations$(COLOR_RESET)"; exit 1; }

atlas-inspect: atlas-install ## Inspect current database schema
	@echo "$(COLOR_BLUE)Inspecting database schema...$(COLOR_RESET)"
	atlas schema inspect --env $(ATLAS_ENV)

atlas-k8s-config: ## Generate Kubernetes ConfigMap with Atlas configuration
	@echo "$(COLOR_BLUE)Generating Kubernetes ConfigMap...$(COLOR_RESET)"
	./scripts/generate-k8s-configmap.sh

##@ Services
.PHONY: services services-stop services-restart logs
services: ## Start database services (PostgreSQL + Redis)
ifdef CI
	@echo "$(COLOR_YELLOW)Running in CI environment - skipping local service startup$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)GitHub Actions provides services on dynamic ports$(COLOR_RESET)"
	@# In CI, services are already provided by GitHub Actions
	@# Just verify they're available
	@if [ -n "$(DATABASE_URL)" ]; then \
		echo "$(COLOR_GREEN)Database URL: $(DATABASE_URL)$(COLOR_RESET)"; \
	fi
	@if [ -n "$(REDIS_URL)" ]; then \
		echo "$(COLOR_GREEN)Redis URL: $(REDIS_URL)$(COLOR_RESET)"; \
	fi
else
	@echo "$(COLOR_BLUE)Starting database services...$(COLOR_RESET)"
	$(DOCKER_COMPOSE_CMD) -f $(DOCKER_COMPOSE) up -d
endif

services-stop: ## Stop all services
ifdef CI
	@echo "$(COLOR_YELLOW)CI environment - services managed by GitHub Actions$(COLOR_RESET)"
else
	@echo "$(COLOR_YELLOW)Stopping services...$(COLOR_RESET)"
	$(DOCKER_COMPOSE_CMD) -f $(DOCKER_COMPOSE) down
endif

services-restart: services-stop services ## Restart all services

logs: ## Show service logs
	$(DOCKER_COMPOSE_CMD) -f $(DOCKER_COMPOSE) logs -f

##@ Testing
.PHONY: test test-all test-unit test-integration test-acceptance test-cleanup coverage
test: test-unit ## Run tests (default: unit tests)

test-all: ## Run all tests (unit + integration + acceptance) with cleanup
	@echo "$(COLOR_BLUE)Running all tests...$(COLOR_RESET)"
	@$(MAKE) test-unit
	@$(MAKE) test-integration
	@$(MAKE) test-acceptance
	@echo "$(COLOR_GREEN)All tests completed successfully!$(COLOR_RESET)"

test-unit: ## Run unit tests (fast, no external services)
	@echo "$(COLOR_BLUE)Running unit tests...$(COLOR_RESET)"
	$(GO) test -race -short -coverprofile=coverage.out -covermode=atomic $(PKG)

test-integration: ## Run integration tests (requires services)
	@echo "$(COLOR_BLUE)Running integration tests...$(COLOR_RESET)"
ifndef CI
	@# Local environment - start services
	@$(MAKE) services
endif
	@trap '$(MAKE) test-cleanup' EXIT; \
		$(GO) test -tags=integration -race -coverprofile=coverage-integration.out -covermode=atomic $(PKG) && \
		echo "$(COLOR_GREEN)Integration tests completed!$(COLOR_RESET)"

test-acceptance: ## Run full acceptance tests (auto-manages services)
	@echo "$(COLOR_BLUE)Running acceptance tests...$(COLOR_RESET)"
ifndef CI
	@# Local environment - start services
	@$(MAKE) services
endif
	@trap '$(MAKE) test-cleanup' EXIT; \
		$(GO) test -tags=acceptance -race -v ./internal/http/acceptance && \
		echo "$(COLOR_GREEN)Acceptance tests completed!$(COLOR_RESET)"

test-cleanup: ## Clean up test resources (test data and artifacts only)
	@echo "$(COLOR_YELLOW)Cleaning up test resources...$(COLOR_RESET)"
ifndef CI
	@# Local environment - clean up using docker
	@docker exec postgres psql -U postgres -d bonsai_test -c "DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public;" 2>/dev/null || true
	@docker exec redis redis-cli FLUSHALL 2>/dev/null || true
endif
	@# Always clean up test artifacts
	@rm -f coverage*.out coverage*.html 2>/dev/null || true
	@echo "$(COLOR_GREEN)Test cleanup completed!$(COLOR_RESET)"

coverage: ## Generate test coverage report
	@echo "$(COLOR_BLUE)Generating coverage report...$(COLOR_RESET)"
	$(GO) test -race -coverprofile=coverage.out -covermode=atomic -short $(PKG)
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "$(COLOR_GREEN)Coverage report: coverage.html$(COLOR_RESET)"

##@ Code Quality
.PHONY: lint format check
format: ## Format code
	@echo "$(COLOR_BLUE)Formatting code...$(COLOR_RESET)"
	$(GO) fmt $(PKG)

lint: format ## Run linter
	@echo "$(COLOR_BLUE)Running linter...$(COLOR_RESET)"
	@command -v golangci-lint >/dev/null || (echo "$(COLOR_YELLOW)Installing golangci-lint...$(COLOR_RESET)" && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run

check: lint test ## Full code check (lint + test)
	@echo "$(COLOR_GREEN)All checks passed!$(COLOR_RESET)"

##@ Docker
.PHONY: docker docker-run
docker: ## Build Docker image
	@echo "$(COLOR_BLUE)Building Docker image...$(COLOR_RESET)"
	docker build -t $(BINARY):latest .

docker-run: docker services ## Run in Docker with services
	docker run --rm --network host -e DATABASE_URL=postgres://postgres:postgres@localhost:5432/bonsai \
		-e REDIS_URL=redis://localhost:6379 $(BINARY):latest

##@ Utilities
.PHONY: health deps-update
health: ## Check API health endpoints
	@echo "$(COLOR_BLUE)Checking health endpoints...$(COLOR_RESET)"
	@curl -s http://localhost:8080/v1/health | jq . 2>/dev/null || echo "API not running or jq not installed"
	@curl -s http://localhost:8080/v1/livez | jq . 2>/dev/null || true
	@curl -s http://localhost:8080/v1/readyz | jq . 2>/dev/null || true

deps-update: ## Update Go dependencies
	@echo "$(COLOR_BLUE)Updating dependencies...$(COLOR_RESET)"
	$(GO) get -u ./...
	$(GO) mod tidy

##@ Help
.PHONY: help commands
help: ## Show this help message
	@echo "$(COLOR_GREEN)"
	@echo "  ____                        _ "
	@echo " | __ )  ___  _ __  ___  __ _(_)"
	@echo " |  _ \ / _ \| '_ \/ __|/ _\` | |"
	@echo " | |_) | (_) | | | \__ \ (_| | |"
	@echo " |____/ \___/|_| |_|___/\__,_|_|"
	@echo ""
	@echo "$(COLOR_RESET)$(COLOR_BLUE)Snippet API Development Tools$(COLOR_RESET)"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make $(COLOR_BLUE)<command>$(COLOR_RESET)\n\nCommands:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  $(COLOR_BLUE)%-15s$(COLOR_RESET) %s\n", $$1, $$2 } /^##@/ { printf "\n$(COLOR_YELLOW)%s$(COLOR_RESET)\n", substr($$0, 5) }' $(MAKEFILE_LIST)
	@echo ""
	@echo "$(COLOR_GREEN)Quick Start:$(COLOR_RESET)"
	@echo "  1. $(COLOR_BLUE)make setup$(COLOR_RESET)    # First time setup"
	@echo "  2. $(COLOR_BLUE)make run$(COLOR_RESET)      # Start the API"
	@echo "  3. $(COLOR_BLUE)make test$(COLOR_RESET)     # Run tests"
	@echo ""

commands: help ## Alias for help