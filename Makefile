# Bonsai API Makefile
# Simple commands to develop, test, and deploy the Bonsai snippet service

# Variables
GO := go
PKG := ./...
BINARY := bonsai
DOCKER_COMPOSE := docker/docker-compose.yaml

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

clean: ## Clean up built artifacts
	@echo "$(COLOR_YELLOW)Cleaning up...$(COLOR_RESET)"
	rm -f $(BINARY)
	rm -f coverage*.out
	$(GO) clean

##@ Services
.PHONY: services services-stop services-restart logs
services: ## Start database services (PostgreSQL + Redis)
	@echo "$(COLOR_BLUE)Starting database services...$(COLOR_RESET)"
	docker compose -f $(DOCKER_COMPOSE) up -d

services-stop: ## Stop all services
	@echo "$(COLOR_YELLOW)Stopping services...$(COLOR_RESET)"
	docker compose -f $(DOCKER_COMPOSE) down

services-restart: services-stop services ## Restart all services

logs: ## Show service logs
	docker compose -f $(DOCKER_COMPOSE) logs -f

##@ Testing
.PHONY: test test-unit test-integration test-acceptance coverage
test: test-unit ## Run tests (default: unit tests)

test-unit: ## Run unit tests (fast, no external services)
	@echo "$(COLOR_BLUE)Running unit tests...$(COLOR_RESET)"
	$(GO) test -race -short $(PKG)

test-integration: services ## Run integration tests (requires services)
	@echo "$(COLOR_BLUE)Running integration tests...$(COLOR_RESET)"
	$(GO) test -tags=integration -race $(PKG)

test-acceptance: ## Run full acceptance tests (auto-manages services)
	@echo "$(COLOR_BLUE)Running acceptance tests...$(COLOR_RESET)"
	$(GO) test -race -v ./internal/http/acceptance

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