
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

### Installation
Clone the repository:
```sh
git clone https://github.com/roguepikachu/bonsai.git
```
Build the project:
```sh
make bonsai-build
```

### Usage
Run the server:
```sh
DOTENV_PATHS=.env ./bonsai
```
Visit `http://localhost:8080/v1/health` for a quick health check.

## Documentation
See the [docs/](docs/) folder for detailed API documentation, architecture, and contribution guidelines.

## Contributing
Pull requests are welcome! For major changes, please open an issue first to discuss what you would like to change.

See [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) for more information.

## License
This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Configuration

Environment variables:

- BONSAI_PORT: API port (default 8080)
- REDIS_PORT: Redis address in host:port (default :6379)
- POSTGRES_URL: Full DSN, e.g. postgres://user:pass@host:5432/db?sslmode=disable
- POSTGRES_HOST, POSTGRES_PORT, POSTGRES_USER, POSTGRES_PASSWORD, POSTGRES_DB, POSTGRES_SSLMODE: used if POSTGRES_URL is not set
- AUTO_MIGRATE: if true, creates the minimal schema on startup
