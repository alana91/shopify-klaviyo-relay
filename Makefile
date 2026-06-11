.PHONY: fmt lint tidy check build test test-docker up db-reset seed-db send-webhook help

ifneq (,$(wildcard .env))
include .env
export
endif

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(firstword $(MAKEFILE_LIST)) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-16s %s\n", $$1, $$2}'

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

up: ## Build and start docker-compose services
	docker-compose up -d --build

db-reset: ## Drop and recreate the local dev database (schema re-applied on next app start)
	docker compose exec -T db psql -U $${DB_USER} -d postgres -c "DROP DATABASE IF EXISTS $${DB_NAME} WITH (FORCE);"
	docker compose exec -T db psql -U $${DB_USER} -d postgres -c "CREATE DATABASE $${DB_NAME};"

seed-db: ## Seed demo data into the DB container
	docker-compose exec db psql -U $${DB_USER} $${DB_NAME} < store/seed.sql

send-webhook: ## Send a fake Shopify webhook to the local server
	bash scripts/send_webhook.sh

send-bad-webhook: ## Send a webhook with no email (expected to fail at Klaviyo)
	bash scripts/send_bad_webhook.sh
