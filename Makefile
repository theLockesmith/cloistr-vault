# Coldforge Vault Makefile

# Variables
BINARY_NAME=vault-api
GO_VERSION=1.21
DOCKER_IMAGE=coldforge/vault
VERSION?=latest

# Go related variables
GOBASE=$(shell pwd)
GOBIN=$(GOBASE)/bin
GOFILES=$(wildcard backend/**/*.go)

# Docker and deployment
DOCKER_COMPOSE=docker-compose
KUBECTL=kubectl
NAMESPACE=coldforge-vault

# Colors for output
RED=\033[0;31m
GREEN=\033[0;32m
YELLOW=\033[1;33m
BLUE=\033[0;34m
NC=\033[0m # No Color

.PHONY: help build test clean run docker-build docker-run deploy-docker deploy-k8s

## help: Show this help message
help:
	@echo "$(BLUE)Coldforge Vault - Available Commands$(NC)"
	@echo ""
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## build: Build the Go application
build:
	@echo "$(YELLOW)Building $(BINARY_NAME)...$(NC)"
	@cd backend && GOOS=linux GOARCH=amd64 go build -o ../$(GOBIN)/$(BINARY_NAME) ./cmd/server
	@echo "$(GREEN)✅ Build completed: $(GOBIN)/$(BINARY_NAME)$(NC)"

## test: Run all tests
test:
	@echo "$(YELLOW)Running tests...$(NC)"
	@cd backend && go test -v ./...
	@echo "$(GREEN)✅ Tests completed$(NC)"

## test-coverage: Run tests with coverage
test-coverage:
	@echo "$(YELLOW)Running tests with coverage...$(NC)"
	@cd backend && go test -coverprofile=coverage.out ./...
	@cd backend && go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✅ Coverage report generated: backend/coverage.html$(NC)"

## test-crypto: Run crypto package tests only
test-crypto:
	@echo "$(YELLOW)Running crypto tests...$(NC)"
	@cd backend && go test -v ./internal/crypto
	@echo "$(GREEN)✅ Crypto tests completed$(NC)"

## clean: Clean build artifacts
clean:
	@echo "$(YELLOW)Cleaning...$(NC)"
	@rm -rf $(GOBIN)
	@rm -f backend/coverage.out backend/coverage.html
	@echo "$(GREEN)✅ Clean completed$(NC)"

## run: Run the application locally
run:
	@echo "$(YELLOW)Starting $(BINARY_NAME)...$(NC)"
	@cd backend && go run ./cmd/server

## deps: Download and tidy Go dependencies
deps:
	@echo "$(YELLOW)Downloading dependencies...$(NC)"
	@cd backend && go mod tidy && go mod download
	@echo "$(GREEN)✅ Dependencies updated$(NC)"

## lint: Run linter (requires golangci-lint)
lint:
	@echo "$(YELLOW)Running linter...$(NC)"
	@cd backend && golangci-lint run
	@echo "$(GREEN)✅ Linting completed$(NC)"

## fmt: Format Go code
fmt:
	@echo "$(YELLOW)Formatting code...$(NC)"
	@cd backend && go fmt ./...
	@echo "$(GREEN)✅ Code formatted$(NC)"

## vet: Run go vet
vet:
	@echo "$(YELLOW)Running go vet...$(NC)"
	@cd backend && go vet ./...
	@echo "$(GREEN)✅ Vet completed$(NC)"

## docker-build: Build Docker image
docker-build:
	@echo "$(YELLOW)Building Docker image $(DOCKER_IMAGE):$(VERSION)...$(NC)"
	@docker build -t $(DOCKER_IMAGE):$(VERSION) .
	@docker tag $(DOCKER_IMAGE):$(VERSION) $(DOCKER_IMAGE):latest
	@echo "$(GREEN)✅ Docker image built$(NC)"

## docker-run: Run application with Docker Compose
docker-run:
	@echo "$(YELLOW)Starting application with Docker Compose...$(NC)"
	@$(DOCKER_COMPOSE) up --build -d
	@echo "$(GREEN)✅ Application started at http://localhost:8080$(NC)"

## docker-stop: Stop Docker Compose services
docker-stop:
	@echo "$(YELLOW)Stopping Docker Compose services...$(NC)"
	@$(DOCKER_COMPOSE) down
	@echo "$(GREEN)✅ Services stopped$(NC)"

## docker-logs: Show Docker Compose logs
docker-logs:
	@$(DOCKER_COMPOSE) logs -f

## deploy-docker: Deploy using Docker Compose
deploy-docker:
	@echo "$(YELLOW)Deploying with Docker Compose...$(NC)"
	@./scripts/deploy.sh docker
	@echo "$(GREEN)✅ Docker deployment completed$(NC)"

## deploy-k8s: Deploy to Kubernetes
deploy-k8s:
	@echo "$(YELLOW)Deploying to Kubernetes...$(NC)"
	@./scripts/deploy.sh kubernetes
	@echo "$(GREEN)✅ Kubernetes deployment completed$(NC)"

## status-docker: Show Docker deployment status
status-docker:
	@./scripts/deploy.sh status docker

## status-k8s: Show Kubernetes deployment status
status-k8s:
	@./scripts/deploy.sh status kubernetes

## migrate-up: Run database migrations up
migrate-up:
	@echo "$(YELLOW)Running database migrations...$(NC)"
	@cd backend && go run cmd/migrate/main.go up
	@echo "$(GREEN)✅ Migrations completed$(NC)"

## migrate-down: Run database migrations down
migrate-down:
	@echo "$(YELLOW)Rolling back database migrations...$(NC)"
	@cd backend && go run cmd/migrate/main.go down
	@echo "$(GREEN)✅ Migrations rolled back$(NC)"

## dev: Start development environment
dev: deps docker-run
	@echo "$(GREEN)🚀 Development environment started!$(NC)"
	@echo "$(BLUE)API: http://localhost:8080$(NC)"
	@echo "$(BLUE)Health: http://localhost:8080/api/v1/health$(NC)"

## benchmark: Run benchmark tests
benchmark:
	@echo "$(YELLOW)Running benchmarks...$(NC)"
	@cd backend && go test -bench=. -benchmem ./...
	@echo "$(GREEN)✅ Benchmarks completed$(NC)"

## security-scan: Run security scan (requires gosec)
security-scan:
	@echo "$(YELLOW)Running security scan...$(NC)"
	@cd backend && gosec ./...
	@echo "$(GREEN)✅ Security scan completed$(NC)"

## generate-keys: Generate sample Nostr keypair for testing
generate-keys:
	@echo "$(YELLOW)Generating sample Nostr keypair...$(NC)"
	@cd backend && go run ./cmd/keygen
	@echo "$(GREEN)✅ Keys generated$(NC)"

## api-docs: Generate API documentation
api-docs:
	@echo "$(YELLOW)Generating API documentation...$(NC)"
	@echo "📋 API endpoints available at /api/v1/info"
	@echo "$(GREEN)✅ See README.md for full documentation$(NC)"

## all: Run formatter, vet, linter, and tests
all: fmt vet lint test
	@echo "$(GREEN)✅ All checks completed$(NC)"

# Default target
.DEFAULT_GOAL := help