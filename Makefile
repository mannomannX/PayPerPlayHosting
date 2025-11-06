.PHONY: help build run dev test clean docker-pull

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the application
	go build -o payperplay ./cmd/api

run: build ## Build and run the application
	./payperplay

dev: ## Run in development mode with auto-reload (requires air)
	air

test: ## Run tests
	go test -v ./...

clean: ## Clean build artifacts
	rm -f payperplay
	rm -f payperplay.db
	rm -rf minecraft/servers/*

deps: ## Download dependencies
	go mod download
	go mod tidy

docker-pull: ## Pull required Docker images
	docker pull itzg/minecraft-server:latest

init: deps docker-pull ## Initialize project (first time setup)
	cp .env.example .env
	@echo "Setup complete! Edit .env if needed, then run 'make run'"

.DEFAULT_GOAL := help
