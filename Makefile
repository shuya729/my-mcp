.PHONY: help up down logs run format test check build

.DEFAULT_GOAL := help

help:
	@echo "Available commands:"
	@echo "  make help   Show this help"
	@echo "  make up     Start PostgreSQL and Logto"
	@echo "  make down   Stop PostgreSQL and Logto"
	@echo "  make logs   Follow PostgreSQL and Logto logs"
	@echo "  make run    Run the MCP server using .env"
	@echo "  make format Format Go code"
	@echo "  make test   Run Go tests"
	@echo "  make check  Run go vet"
	@echo "  make build  Build bin/mcp"

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

run:
	@set -a; . ./.env; set +a; go run ./cmd/mcp

format:
	go fmt ./...

test:
	go test ./...

check:
	go vet ./...

build:
	@mkdir -p bin
	go build -o bin/mcp ./cmd/mcp
