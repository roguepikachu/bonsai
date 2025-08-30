# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o bonsai ./cmd/api/main.go

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/bonsai /app/bonsai
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=2s --retries=3 CMD wget -qO- http://127.0.0.1:8080/v1/readyz || exit 1
ENTRYPOINT ["/app/bonsai"]
