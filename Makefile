.PHONY: help up down logs run run-mcp run-api migrate migrate-status format test check build build-mcp build-api

.DEFAULT_GOAL := help

help:
	@echo "Available commands:"
	@echo "  make help   Show this help"
	@echo "  make up     Start PostgreSQL and Logto"
	@echo "  make down   Stop PostgreSQL and Logto"
	@echo "  make logs   Follow PostgreSQL and Logto logs"
	@echo "  make run    Run the MCP server and REST API using .env"
	@echo "  make run-mcp Run the MCP server using .env"
	@echo "  make run-api Run the REST API using .env"
	@echo "  make migrate Apply app database migrations"
	@echo "  make migrate-status Show app database migration status"
	@echo "  make format Format Go code"
	@echo "  make test   Run Go tests"
	@echo "  make check  Run go vet"
	@echo "  make build  Build bin/mcp and bin/api"
	@echo "  make build-mcp Build bin/mcp"
	@echo "  make build-api Build bin/api"

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

run:
	@$(MAKE) -j2 run-mcp run-api

run-mcp:
	@set -a; . ./.env; set +a; go run ./cmd/mcp

run-api:
	@set -a; . ./.env; set +a; go run ./cmd/api

migrate:
	@set -a; . ./.env; set +a; : "$${POSTGRES_USER:?POSTGRES_USER is required}"; : "$${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}"; : "$${APP_DB:?APP_DB is required}"; PGHOST=127.0.0.1 PGPORT=5432 PGUSER="$$POSTGRES_USER" PGPASSWORD="$$POSTGRES_PASSWORD" PGDATABASE="$$APP_DB" PGSSLMODE=disable go tool goose -dir postgres/migrations postgres "" up

migrate-status:
	@set -a; . ./.env; set +a; : "$${POSTGRES_USER:?POSTGRES_USER is required}"; : "$${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}"; : "$${APP_DB:?APP_DB is required}"; PGHOST=127.0.0.1 PGPORT=5432 PGUSER="$$POSTGRES_USER" PGPASSWORD="$$POSTGRES_PASSWORD" PGDATABASE="$$APP_DB" PGSSLMODE=disable go tool goose -dir postgres/migrations postgres "" status

format:
	go fmt ./...

test:
	go test ./...

check:
	go vet ./...

build: build-mcp build-api

build-mcp:
	@mkdir -p bin
	go build -o bin/mcp ./cmd/mcp

build-api:
	@mkdir -p bin
	go build -o bin/api ./cmd/api
