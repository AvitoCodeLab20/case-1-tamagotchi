.PHONY: up down restart rebuild logs ps test build lint fmt-check migrate migration-status smoke clean help

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

migration-status: ## Show applied database migrations
	docker compose exec postgres psql -U "$${POSTGRES_USER:-postgres}" -d "$${POSTGRES_DB:-tamagotchi}" -c "TABLE schema_migrations;"

smoke: ## Check backend liveness and readiness endpoints
	curl --fail --silent http://localhost:$${BACKEND_PORT:-8080}/healthz
	curl --fail --silent http://localhost:$${BACKEND_PORT:-8080}/readyz

# --- Go backend ---

test: ## Run backend tests with race detector
	cd backend && go test -race ./...

build: ## Compile the backend locally
	cd backend && go build ./cmd

lint: ## Run the configured backend linters
	cd backend && golangci-lint run --config ../.golangci.yaml

fmt-check: ## Check Go formatting
	test -z "$$(gofmt -l backend)"

clean: ## Stop containers and remove the local database
	docker compose down -v

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
