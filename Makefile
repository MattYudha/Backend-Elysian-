ifneq (,$(wildcard .env))
    include .env
    export
endif

.PHONY: help run build test clean docker-up docker-down swagger

DB_URL="postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSL_MODE)"

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

swagger: ## Generate Swagger documentation
	swag init -g cmd/server/main.go

run: ## Run the application
	go run cmd/server/main.go

build: ## Build the application
	go build -o bin/server cmd/server/main.go

test: ## Run tests
	go test -v ./...

clean: ## Clean build artifacts
	rm -rf bin/ tmp/

docker-up: ## Start Docker services
	docker-compose up -d

docker-down: ## Stop Docker services
	docker-compose down

docker-logs: ## View Docker logs
	docker-compose logs -f

migrate-up: ## Run migrations
	goose -dir migrations postgres $(DB_URL) up

migrate-down: ## Rollback last migration
	goose -dir migrations postgres $(DB_URL) down

migrate-status: ## Check migration status
	goose -dir migrations postgres $(DB_URL) status

migrate-reset: ## Reset all migrations
	goose -dir migrations postgres $(DB_URL) reset

migrate-create: ## Create new migration (use: make migrate-create NAME=migration_name)
	goose -dir migrations create $(NAME) sql

.DEFAULT_GOAL := help
