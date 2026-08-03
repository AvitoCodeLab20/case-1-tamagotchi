.PHONY: up down restart rebuild logs ps test build lint clean

# --- Docker ---

up: ## Start everything (build if needed)
	docker compose up --build -d

down: ## Stop everything
	docker compose down

restart: down up ## Restart everything

rebuild: ## Full rebuild: stop, wipe db volume, build, start
	docker compose down -v
	docker compose up --build -d

logs: ## Tail logs of all services
	docker compose logs -f

ps: ## Show container status
	docker compose ps

# --- Go (backend) ---

test: ## Run all Go tests
	cd backend && go test ./...

build: ## Compile the Go service locally
	cd backend && go build ./...

lint: ## Run go vet
	cd backend && go vet ./...

clean: ## Stop containers and remove db volume
	docker compose down -v

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'
