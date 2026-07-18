.PHONY: help build test test-unit test-property lint fmt vet clean docker-build verify

# Variables
APP_NAME=signer-service
BINARY_NAME=signer-service
DOCKER_IMAGE=platform/$(APP_NAME):latest
GO=go
GOFLAGS=-v
LDFLAGS=-ldflags "-s -w"
GO_FILES=$(shell find . -name '*.go' -not -path './vendor/*')

# Colors for output
COLOR_RESET=\033[0m
COLOR_BOLD=\033[1m
COLOR_GREEN=\033[32m
COLOR_YELLOW=\033[33m

help: ## Display this help message
	@echo "$(COLOR_BOLD)Available targets:$(COLOR_RESET)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(COLOR_GREEN)%-15s$(COLOR_RESET) %s\n", $$1, $$2}'

build: ## Build the application binary
	@echo "$(COLOR_YELLOW)Building $(APP_NAME)...$(COLOR_RESET)"
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/
	@echo "$(COLOR_GREEN)Build complete: bin/$(BINARY_NAME)$(COLOR_RESET)"

test: ## Run all tests
	@echo "$(COLOR_YELLOW)Running tests...$(COLOR_RESET)"
	$(GO) test $(GOFLAGS) -race -count=1 -coverprofile=coverage.out ./...
	@echo "$(COLOR_GREEN)Tests complete$(COLOR_RESET)"

test-unit: ## Run unit tests only
	@echo "$(COLOR_YELLOW)Running unit tests...$(COLOR_RESET)"
	$(GO) test $(GOFLAGS) -race -count=1 -run 'Test[^P]' ./...
	@echo "$(COLOR_GREEN)Unit tests complete$(COLOR_RESET)"

test-property: ## Run property-based tests only
	@echo "$(COLOR_YELLOW)Running property-based tests...$(COLOR_RESET)"
	$(GO) test $(GOFLAGS) -race -count=1 -run 'TestProperty' ./...
	@echo "$(COLOR_GREEN)Property tests complete$(COLOR_RESET)"

lint: ## Run linter
	@echo "$(COLOR_YELLOW)Running linter...$(COLOR_RESET)"
	golangci-lint run --timeout=5m ./...
	@echo "$(COLOR_GREEN)Linting complete$(COLOR_RESET)"

fmt: ## Format code
	@echo "$(COLOR_YELLOW)Formatting code...$(COLOR_RESET)"
	gofmt -s -w $(GO_FILES)
	@echo "$(COLOR_GREEN)Formatting complete$(COLOR_RESET)"

vet: ## Run go vet
	@echo "$(COLOR_YELLOW)Running go vet...$(COLOR_RESET)"
	$(GO) vet ./...
	@echo "$(COLOR_GREEN)Vet complete$(COLOR_RESET)"

clean: ## Clean build artifacts
	@echo "$(COLOR_YELLOW)Cleaning...$(COLOR_RESET)"
	rm -rf bin/
	rm -f coverage.out coverage.html
	$(GO) clean -cache
	@echo "$(COLOR_GREEN)Clean complete$(COLOR_RESET)"

docker-build: ## Build Docker image
	@echo "$(COLOR_YELLOW)Building Docker image...$(COLOR_RESET)"
	docker build -t $(DOCKER_IMAGE) .
	@echo "$(COLOR_GREEN)Docker image built: $(DOCKER_IMAGE)$(COLOR_RESET)"

verify: fmt vet lint test ## Run all verification steps (format, vet, lint, test)
	@echo "$(COLOR_GREEN)All verification steps passed$(COLOR_RESET)"

.DEFAULT_GOAL := help
