# Makefile for Bonsai project

.PHONY: lint

lint:
	golangci-lint run
