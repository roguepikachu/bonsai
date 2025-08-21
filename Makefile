# Makefile for Bonsai project

.PHONY: lint

lint: fmt
	golangci-lint run

fmt: 
	go fmt ./...