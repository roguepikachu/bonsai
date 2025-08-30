# Testing

This project separates unit and integration tests using build tags.

- Unit tests: default `go test ./...` run; fast, no external deps.
- Integration tests: use the `integration` build tag, may start embedded/test servers.

## Commands

Run unit tests only:

```
make test-unit
```

Run integration tests (and unit):

```
make test-integration
```

## Notes

- The cached repository integration test uses `miniredis` to avoid a real Redis instance.
- For future DB integration tests, prefer `testcontainers-go` or a docker-compose profile.
