.PHONY: help demo build clean setup-check test-webhook start-services stop-services

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

demo: ## Run the interactive demo
	go run ./cmd/demo/main.go

build: ## Build the demo binary
	go build -o demo_bin ./cmd/demo
	@echo "Binary created: ./demo_bin"

clean: ## Clean build artifacts
	rm -f demo_bin
	rm -f generation_service.pid
	rm -f generation_service.log

setup-check: ## Check if all required services are running
	./demo/setup-check.sh

test-webhook: ## Send a test webhook (requires demo to be running)
	./demo/test-webhook.sh

start-services: ## Start all required services (ChromaDB and Generation Service)
	./demo/start-services.sh

stop-services: ## Stop all demo services
	./demo/stop-services.sh

run-main: ## Run the main BrainBot API server
	go run main.go

build-main: ## Build the main BrainBot API binary
	go build -o brainbot .
