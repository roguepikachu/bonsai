# Makefile for Bonsai project
DOCKER_COMPOSE = docker-compose -f docker/docker-compose.yaml

.PHONY: lint

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