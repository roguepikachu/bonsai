DOCKER_COMPOSE="docker/docker-compose.yaml"
DOTENV_PATHS=".env"

lint: fmt
	golangci-lint run

fmt: 
	go fmt ./...

.PHONY: redis-up redis-down redis-logs redis-restart

redis-up:
	$(DOCKER_COMPOSE) up -d redis

redis-down:
	$(DOCKER_COMPOSE) down

redis-logs:
	$(DOCKER_COMPOSE) logs -f redis

redis-restart: redis-down redis-up

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