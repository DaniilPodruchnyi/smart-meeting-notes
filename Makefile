.PHONY: dev build up down logs clean test lint

dev:
	cp -n .env.example .env 2>/dev/null || true
	docker-compose up --build

up:
	docker-compose up -d

down:
	docker-compose down

logs:
	docker-compose logs -f

build:
	docker-compose build

clean:
	docker-compose down -v
	rm -f .env

test:
	go test -v ./...

lint:
	golangci-lint run ./...
	@staticcheck ./...

run-local:
	go run ./cmd/server

build-binary:
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.buildVersion=$$(git describe --tags --always --dirty) -X main.buildDate=$$(date -u +%Y-%m-%d) -X main.buildCommit=$$(git rev-parse HEAD)" -o bin/server ./cmd/server

help:
	@echo "Available commands:"
	@echo "  make dev         - Start development environment (docker-compose)"
	@echo "  make up          - Start containers in background"
	@echo "  make down        - Stop containers"
	@echo "  make logs        - View all logs"
	@echo "  make build       - Build containers"
	@echo "  make clean       - Stop and remove volumes"
	@echo "  make test        - Run tests"
	@echo "  make lint        - Run linters"
	@echo "  make run-local   - Run locally (not in docker)"
	@echo "  make build-binary - Build production binary"