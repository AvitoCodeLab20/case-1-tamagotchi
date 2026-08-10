.PHONY: up down restart rebuild logs ps test test-integration build lint fmt-check migrate migration-status smoke clean help

# --- Docker ---

up: ## Build and start PostgreSQL, migrations, and backend
	docker compose up --build -d

down: ## Stop all services
	docker compose down

restart: down up ## Restart all services

rebuild: ## Recreate services and the local database
	docker compose down -v
	docker compose up --build -d

logs: ## Follow service logs
	docker compose logs -f

ps: ## Show container status
	docker compose ps

migrate: ## Apply pending database migrations
	docker compose run --rm migrate

migration-status: ## Show goose migration status
	docker compose run --rm migrate status

smoke: ## Check backend liveness and readiness endpoints
	curl --fail --silent http://localhost:$${BACKEND_PORT:-8080}/healthz
	curl --fail --silent http://localhost:$${BACKEND_PORT:-8080}/readyz

# --- Go backend ---

test: ## Run backend tests with race detector
	cd backend && go test -race ./...

test-integration: ## Run backend tests against the Compose database (requires `make up`)
	cd backend && TEST_DATABASE_URL="postgres://$${POSTGRES_USER:-postgres}:$${POSTGRES_PASSWORD:-postgres}@localhost:$${POSTGRES_PORT:-5433}/$${POSTGRES_DB:-tamagotchi}?sslmode=disable" \
		go test -race -count=1 ./...

build: ## Compile backend binaries locally into backend/bin
	cd backend && go build -o bin/ ./cmd/...

lint: ## Run the configured backend linters
	cd backend && golangci-lint run --config ../.golangci.yaml

fmt-check: ## Check Go formatting
	test -z "$$(gofmt -l backend)"

clean: ## Stop containers and remove the local database
	docker compose down -v

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
