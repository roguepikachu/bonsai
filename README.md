
# Bonsai

<p align="center">
  <img src="img/bonsai.png" alt="Bonsai logo" width="300"/>
</p>

Bonsai is a fast, lightweight, and scalable snippet store written in Go. It lets you create short text snippets with optional expiry and tags.

## Features
- Simple and clean API for creating and managing short URLs
- In-memory and persistent storage options
- Expiry and cache management
- Analytics and rate limiting
- Real-time pub/sub updates
- Stampede guard for high-traffic protection

## Getting Started

### Prerequisites
- Go 1.22 or higher
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

## Documentation
See the [docs/](docs/) folder for detailed API documentation, architecture, and contribution guidelines.

## Contributing
Pull requests are welcome! For major changes, please open an issue first to discuss what you would like to change.

See [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) for more information.

## License
This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Configuration

Copy `.env.example` to `.env` and adjust as needed. Key variables:

- BONSAI_PORT: API port (default 8080)
- REDIS_PORT: Redis address in host:port (default :6379)
- POSTGRES_URL: Full DSN, e.g. postgres://user:pass@host:5432/db?sslmode=disable
- POSTGRES_HOST, POSTGRES_PORT, POSTGRES_USER, POSTGRES_PASSWORD, POSTGRES_DB, POSTGRES_SSLMODE: used if POSTGRES_URL is not set
- AUTO_MIGRATE: if true, creates the minimal schema on startup
- LOG_LEVEL: trace|debug|info|warn|error (default debug)
- LOG_FORMAT: text|json (default text)
