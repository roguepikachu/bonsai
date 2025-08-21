# syntax=docker/dockerfile:1
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o bonsai ./cmd/api/main.go

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/bonsai /app/bonsai
EXPOSE 8080
ENTRYPOINT ["/app/bonsai"]
