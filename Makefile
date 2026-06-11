.PHONY: fmt lint tidy check build test test-docker generate up seed-db send-webhook help

ifneq (,$(wildcard .env))
include .env
export
endif

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-16s %s\n", $$1, $$2}'

fmt: ## Format Go source files
	go fmt ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

tidy: ## Tidy go.mod and go.sum
	go mod tidy

check: fmt tidy lint ## Format, tidy, and lint

build: ## Build binary to bin/relay
	go build -o bin/relay .

test: ## Run tests with race detector (local; Postgres reached at localhost)
	DB_HOST=localhost go test -race ./...

test-docker: ## Run tests in a container on the compose network (Postgres reached at db)
	docker compose run --rm test

generate: ## Run sqlc code generation
	sqlc generate

up: ## Build and start docker-compose services
	docker-compose up -d --build

seed-db: ## Seed demo data into the DB container
	docker-compose exec db psql -U $${DB_USER} $${DB_NAME} < store/seed.sql

send-webhook: ## Send a fake Shopify webhook to the local server
	bash scripts/send_webhook.sh
