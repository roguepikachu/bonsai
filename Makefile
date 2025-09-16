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
	POSTGRES_URL="postgres://postgres:postgres@localhost:5432/bonsai?sslmode=disable" \
	 REDIS_URL="redis://localhost:6379" \
	 $(GO) run ./cmd/api/main.go

dev: ## Start the development environment (services + migrations + API server)
	@echo "$(COLOR_GREEN)Starting Bonsai development environment...$(COLOR_RESET)"
	@$(MAKE) services
	@echo "$(COLOR_YELLOW)Waiting for database to be ready...$(COLOR_RESET)"
	@for i in $$(seq 1 30); do \
		if docker exec postgres pg_isready -U postgres >/dev/null 2>&1; then \
			echo "$(COLOR_GREEN)Database is ready!$(COLOR_RESET)"; \
			break; \
		fi; \
		echo "$(COLOR_YELLOW)Database not ready, waiting ($$i/30)...$(COLOR_RESET)"; \
		sleep 2; \
		if [ $$i -eq 30 ]; then \
			echo "$(COLOR_RED)Database failed to become ready$(COLOR_RESET)"; \
			exit 1; \
		fi; \
	done
	@DATABASE_URL="postgres://postgres:postgres@localhost:5432/bonsai?sslmode=disable" $(MAKE) migrate-apply
	@echo "$(COLOR_BLUE)Starting API server...$(COLOR_RESET)"
	@POSTGRES_URL="postgres://postgres:postgres@localhost:5432/bonsai?sslmode=disable" \
	 REDIS_URL="redis://localhost:6379" \
	 $(GO) run ./cmd/api/main.go

##@ Services
.PHONY: services services-stop logs
services: ## Start database services (PostgreSQL + Redis)
ifdef CI
	@echo "$(COLOR_YELLOW)Running in CI environment - skipping local service startup$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)GitHub Actions provides services on dynamic ports$(COLOR_RESET)"
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

logs: ## Show service logs
	$(DOCKER_COMPOSE_CMD) -f $(DOCKER_COMPOSE) logs -f

##@ Database Migrations
.PHONY: migrate-install migrate-apply migrate-status
ATLAS_ENV ?= local

migrate-install: ## Install Atlas CLI
	@echo "$(COLOR_BLUE)Installing Atlas CLI...$(COLOR_RESET)"
	@command -v atlas >/dev/null 2>&1 || { \
		echo "$(COLOR_YELLOW)Atlas not found, installing...$(COLOR_RESET)"; \
		curl -sSf https://atlasgo.sh | sh; \
	}
	@echo "$(COLOR_GREEN)Atlas CLI installed successfully!$(COLOR_RESET)"

migrate-apply: migrate-install ## Apply pending migrations to database
	@echo "$(COLOR_BLUE)Applying migrations...$(COLOR_RESET)"
	atlas migrate apply --env $(ATLAS_ENV) --allow-dirty
	@echo "$(COLOR_GREEN)Migrations applied successfully!$(COLOR_RESET)"

migrate-status: migrate-install ## Show migration status
	@echo "$(COLOR_BLUE)Checking migration status...$(COLOR_RESET)"
	atlas migrate status --env $(ATLAS_ENV)

##@ Testing
.PHONY: test test-all test-unit test-integration test-cleanup
test: test-unit ## Run tests (default: unit tests)

test-all: ## Run all tests (unit + integration) with cleanup
	@echo "$(COLOR_BLUE)Running all tests...$(COLOR_RESET)"
	@$(MAKE) test-unit
	@$(MAKE) test-integration
	@echo "$(COLOR_GREEN)All tests completed successfully!$(COLOR_RESET)"

test-unit: ## Run unit tests (fast, no external services)
	@echo "$(COLOR_BLUE)Running unit tests...$(COLOR_RESET)"
	$(GO) test -race -short -coverprofile=coverage.out -covermode=atomic $(PKG)

test-integration: ## Run integration tests (requires services)
	@echo "$(COLOR_BLUE)Running integration tests...$(COLOR_RESET)"
ifndef CI
	@$(MAKE) services
endif
	@trap '$(MAKE) test-cleanup' EXIT; \
		$(GO) test -tags=integration -race -coverprofile=coverage-integration.out -covermode=atomic $(PKG) && \
		echo "$(COLOR_GREEN)Integration tests completed!$(COLOR_RESET)"

test-cleanup: ## Clean up test resources
	@echo "$(COLOR_YELLOW)Cleaning up test resources...$(COLOR_RESET)"
ifndef CI
	@docker exec postgres psql -U postgres -d bonsai_test -c "DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public;" 2>/dev/null || true
	@docker exec redis redis-cli FLUSHALL 2>/dev/null || true
endif
	@rm -f coverage*.out coverage*.html 2>/dev/null || true
	@echo "$(COLOR_GREEN)Test cleanup completed!$(COLOR_RESET)"

##@ Build & Clean
.PHONY: build clean
build: ## Build the binary
	@echo "$(COLOR_BLUE)Building $(BINARY)...$(COLOR_RESET)"
	$(GO) build -o $(BINARY) ./cmd/api/main.go

clean: ## Clean up built artifacts, test resources, and stop services
	@echo "$(COLOR_YELLOW)Cleaning up...$(COLOR_RESET)"
	rm -f $(BINARY)
	rm -f coverage*.out coverage*.html
	$(GO) clean
	@$(MAKE) test-cleanup
	@$(MAKE) services-stop

##@ API Testing & Deployment
.PHONY: sanity-check setup-and-test health
sanity-check: ## Run comprehensive API sanity tests (requires running API server)
	@echo "$(COLOR_BLUE)Running API sanity check...$(COLOR_RESET)"
	@./scripts/api-sanity-check.sh

test-api: ## Run sanity tests against local API on port 8080
	@echo "$(COLOR_BLUE)Testing API on http://localhost:8080...$(COLOR_RESET)"
	@API_BASE_URL="http://localhost:8080" ./scripts/api-sanity-check.sh

test-simple: ## Run simple API tests (basic functionality)
	@echo "$(COLOR_BLUE)Running simple API tests...$(COLOR_RESET)"
	@./scripts/simple-api-test.sh

setup-and-test: ## Complete setup, migration, start server, and run sanity tests
	@echo "$(COLOR_GREEN)Complete Bonsai setup and test...$(COLOR_RESET)"
	@echo "$(COLOR_BLUE)1. Setting up services...$(COLOR_RESET)"
	@$(MAKE) setup
	@echo "$(COLOR_BLUE)2. Waiting for database and applying migrations...$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Waiting for database to be ready...$(COLOR_RESET)"
	@for i in $$(seq 1 30); do \
		if docker exec postgres pg_isready -U postgres >/dev/null 2>&1; then \
			echo "$(COLOR_GREEN)Database is ready!$(COLOR_RESET)"; \
			break; \
		fi; \
		echo "$(COLOR_YELLOW)Database not ready, waiting ($$i/30)...$(COLOR_RESET)"; \
		sleep 2; \
		if [ $$i -eq 30 ]; then \
			echo "$(COLOR_RED)Database failed to become ready after 60 seconds$(COLOR_RESET)"; \
			exit 1; \
		fi; \
	done
	@DATABASE_URL="postgres://postgres:postgres@localhost:5432/bonsai_test?sslmode=disable" $(MAKE) migrate-apply
	@echo "$(COLOR_BLUE)3. Starting API server in background...$(COLOR_RESET)"
	@echo "$(COLOR_YELLOW)Cleaning up any existing API processes...$(COLOR_RESET)"
	@pkill -f "go run ./cmd/api/main.go" || true
	@sleep 2
	@POSTGRES_URL="postgres://postgres:postgres@localhost:5432/bonsai_test?sslmode=disable" \
	 REDIS_URL="redis://localhost:6379" \
	 BONSAI_PORT=8085 \
	 $(GO) run ./cmd/api/main.go & echo $$! > .api.pid
	@echo "$(COLOR_BLUE)4. Running sanity checks...$(COLOR_RESET)"
	@API_BASE_URL="http://localhost:8085" ./scripts/api-sanity-check.sh || { \
		echo "$(COLOR_YELLOW)Stopping API server...$(COLOR_RESET)"; \
		[ -f .api.pid ] && kill $$(cat .api.pid) 2>/dev/null || true; \
		rm -f .api.pid; \
		exit 1; \
	}
	@echo "$(COLOR_YELLOW)Stopping API server...$(COLOR_RESET)"
	@[ -f .api.pid ] && kill $$(cat .api.pid) 2>/dev/null || true
	@rm -f .api.pid
	@echo "$(COLOR_GREEN)Setup and test completed successfully!$(COLOR_RESET)"

health: ## Check API health endpoints
	@echo "$(COLOR_BLUE)Checking health endpoints...$(COLOR_RESET)"
	@curl -s http://localhost:8080/v1/livez | jq . 2>/dev/null || echo "API not running or jq not installed"
	@curl -s http://localhost:8080/v1/readyz | jq . 2>/dev/null || true

##@ Help
.PHONY: help
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
	@echo "  1. $(COLOR_BLUE)make setup$(COLOR_RESET)      # First time setup"
	@echo "  2. $(COLOR_BLUE)make run$(COLOR_RESET)        # Start the API"
	@echo "  3. $(COLOR_BLUE)make test$(COLOR_RESET)       # Run tests"
	@echo "  4. $(COLOR_BLUE)make clean$(COLOR_RESET)      # Stop and clean up"
	@echo ""