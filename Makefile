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
.PHONY: build
build:
	go build -o bonsai ./cmd/api/main.go

# Run the bonsai server (from source)
.PHONY: run
run:
	go run ./cmd/api/main.go

# Build Docker image
.PHONY: docker-build
docker-build:
	docker build -t bonsai:latest .

# Run bonsai binary (after build)
.PHONY: start
start:
	./bonsai