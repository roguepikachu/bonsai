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