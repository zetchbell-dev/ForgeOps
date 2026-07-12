.PHONY: build test run migrate-up migrate-down docker-build docker-up docker-down lint

SERVICE_BIN := bin/server
MIGRATE_BIN := bin/migrate
COMPOSE     := docker compose -f deployment/docker-compose.yml

build: ## Build server and migrate binaries into bin/
	go build -o $(SERVICE_BIN) ./cmd/server
	go build -o $(MIGRATE_BIN) ./cmd/migrate

test: ## Run all unit + integration tests
	go test ./...

run: build ## Run the service locally (expects POSTGRES_DSN, REDIS_ADDR, JWT_SIGNING_KEY set)
	./$(SERVICE_BIN)

migrate-up: build ## Apply pending Postgres migrations
	./$(MIGRATE_BIN) -direction=up

migrate-down: build ## Roll back the most recent migration
	./$(MIGRATE_BIN) -direction=down

docker-build: ## Build the auth-service image
	$(COMPOSE) build

docker-up: ## Start postgres, redis, run migrations, start auth-service
	$(COMPOSE) up --build

docker-down: ## Tear down the local docker-compose stack
	$(COMPOSE) down -v

lint: ## go vet as a minimal static check (no external linter assumed installed)
	go vet ./...
