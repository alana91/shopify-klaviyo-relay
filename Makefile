.PHONY: fmt lint tidy check build test generate up migrate seed-db send-webhook help

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

test: ## Run tests with race detector
	go test -race ./...

generate: ## Run sqlc code generation
	sqlc generate

up: ## Build and start docker-compose services
	docker-compose up -d --build

migrate: ## Apply schema to the DB container
	docker-compose exec db psql -U $${DB_USER} $${DB_NAME} < store/schema.sql

seed-db: ## Seed demo data into the DB container
	docker-compose exec db psql -U $${DB_USER} $${DB_NAME} < store/seed.sql

send-webhook: ## Send a fake Shopify webhook to the local server
	bash scripts/send_webhook.sh
