DOCKER_COMPOSE="docker/docker-compose.yaml"
DOTENV_PATHS=".env"

lint: fmt
	golangci-lint run

fmt: 
	go fmt ./...

.PHONY: test test-unit test-integration
test: test-unit

test-unit:
	go test ./...

test-integration:
	go test -tags=integration ./...

.PHONY: redis-up redis-down redis-logs redis-restart postgres-up postgres-down postgres-logs postgres-restart dev-up dev-down

redis-up:
	docker compose -f $(DOCKER_COMPOSE) up -d redis

redis-down:
	docker compose -f $(DOCKER_COMPOSE) down redis

redis-logs:
	docker compose -f $(DOCKER_COMPOSE) logs -f redis

redis-restart: redis-down redis-up

postgres-up:
	docker compose -f $(DOCKER_COMPOSE) up -d postgres

postgres-down:
	docker compose -f $(DOCKER_COMPOSE) down postgres

postgres-logs:
	docker compose -f $(DOCKER_COMPOSE) logs -f postgres

postgres-restart: postgres-down postgres-up

dev-up: redis-up postgres-up

dev-down:
	docker compose -f $(DOCKER_COMPOSE) down

# Build the bonsai binary
.PHONY: bonsai-build
bonsai-build:
	go build -o bonsai ./cmd/api/main.go

# Run the bonsai server (from source)
.PHONY: bonsai-run
bonsai-run:
	DOTENV_PATHS=$(DOTENV_PATHS) go run ./cmd/api/main.go

# Build Docker image
.PHONY: bonsai-image
bonsai-image:
	docker build -t bonsai:latest .

# Run bonsai binary (after build)
.PHONY: start
start: bonsai-build
	./bonsai