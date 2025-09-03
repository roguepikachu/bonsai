# Bonsai

<p align="center">
  <img src="img/bonsai.png" alt="Bonsai logo" width="300"/>
</p>

Bonsai is a fast, lightweight text snippet sharing service written in Go. Create and share text/code snippets with optional expiration and tags.

## Features
- Simple REST API for managing text snippets
- Optional expiration (auto-delete after specified time)
- Tag-based organization and search
- Redis caching for fast retrieval
- PostgreSQL persistent storage
- Thread-safe concurrent handling
- Kubernetes-ready health probes

## Getting Started

### Prerequisites
- Go 1.23 or higher
- Docker (for local Redis/Postgres)

### Quickstart (local)
1. Bootstrap env and infra
  ```sh
  make bootstrap
  ```
  This copies `.env.example` to `.env`, starts Redis and Postgres with Docker, and runs `go mod tidy`.

2. Run the API
  ```sh
  make dev
  ```

3. Health checks
  ```sh
  make probes
  ```
  - Liveness: http://localhost:8080/v1/livez
  - Readiness: http://localhost:8080/v1/readyz
  - Legacy: http://localhost:8080/v1/health

### Run everything with one command

If you want DBs and API together in one go:

```sh
make up
```

Stop services:

```sh
make down
```

### Docker image build

Build the API Docker image (customizable name and tag):

```sh
make bonsai-image           # builds bonsai:latest
make IMAGE=myorg/bonsai TAG=v1.2.3 bonsai-image
```

Aliases:

```sh
make image
make docker-image
make docker-build
```

### Make commands

Run `make help` to see all available commands and descriptions. The Makefile is the source of truth for command docs.

## Documentation
- [API Reference](docs/API.md) - Endpoints and examples
- [Architecture](docs/ARCHITECTURE.md) - System design
- [Use Cases](docs/USE_CASES.md) - Real-world examples
- [Testing](docs/TESTING.md) - Test strategy
- [Roadmap](docs/ROADMAP.md) - Future features
- [Contributing](docs/CONTRIBUTING.md) - Contribution guide

## Configuration

Copy `.env.example` to `.env` and adjust as needed. Key variables:

- BONSAI_PORT: API port (default 8080)
- REDIS_PORT: Redis address in host:port (default :6379)
- POSTGRES_URL: Full DSN, e.g. postgres://user:pass@host:5432/db?sslmode=disable
- POSTGRES_HOST, POSTGRES_PORT, POSTGRES_USER, POSTGRES_PASSWORD, POSTGRES_DB, POSTGRES_SSLMODE: used if POSTGRES_URL is not set
- AUTO_MIGRATE: if true, creates the minimal schema on startup
- LOG_LEVEL: trace|debug|info|warn|error (default debug)
- LOG_FORMAT: text|json (default text)

## Contributing
Pull requests are welcome! For major changes, please open an issue first to discuss what you would like to change.

See [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) for more information.

## License
This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.