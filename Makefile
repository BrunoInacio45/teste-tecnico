.PHONY: build test test-integration lint run down swagger help

AGGREGATOR_DIR := services/aggregator
PROCESSOR_DIR  := services/processor

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

build: ## Build both services
	cd $(AGGREGATOR_DIR) && go build ./...
	cd $(PROCESSOR_DIR)  && go build ./...

test: ## Run unit tests for both services
	cd $(AGGREGATOR_DIR) && go test ./...
	cd $(PROCESSOR_DIR)  && go test ./...

test-integration: ## Run integration tests (requires Docker)
	cd $(AGGREGATOR_DIR) && go test -tags integration -v -timeout 120s ./test/integration/...
	cd $(PROCESSOR_DIR)  && go test -tags integration -v -timeout 120s ./test/integration/...

run: ## Start all services with Docker Compose
	docker compose up --build

down: ## Stop all services
	docker compose down
