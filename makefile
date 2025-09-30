# PubSubGo Makefile
# Make configuration
.PHONY: help build test clean lint fmt deps run server docker dev deploy k8s
.DEFAULT_GOAL := help

# Variables
GO := go
BINARY_NAME := pubsubgo-server
CLI_BINARY := pubsubgo-cli
BUILD_DIR := bin
DOCKER_IMAGE := pubsubgo
DOCKER_TAG := latest
COVERAGE_FILE := coverage.out
TEST_TIMEOUT := 300s

# Go build variables
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
CGO_ENABLED ?= 0

# Version variables (can be overridden)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "unknown")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# LDFLAGS for version info
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME) -w -s"

## Display this help message
help:
	@echo "PubSubGo - High-Performance Message Broker"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""
	@echo "Testing targets:"
	@awk 'BEGIN {FS = ":.*?### "} /^[a-zA-Z_-]+:.*?### / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Development targets
## Install project dependencies
deps:
	@echo "Installing dependencies..."
	$(GO) mod download
	$(GO) mod tidy
	$(GO) mod verify

## Format code using gofmt
fmt:
	@echo "Formatting code..."
	gofmt -s -w .
	$(GO) mod tidy

## Run linting (requires golangci-lint)
lint:
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "Warning: golangci-lint not found. Install with: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$$(go env GOPATH)/bin v1.54.2"; \
		echo "Running basic go vet instead..."; \
		$(GO) vet ./...; \
	fi

## Build all binaries
build: build-server build-cli

## Build server binary
build-server:
	@echo "Building server binary..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		$(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server

## Build CLI binary
build-cli:
	@echo "Building CLI binary..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		$(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_BINARY) ./cmd/cli

## Run the server locally
run: build-server
	@echo "Starting PubSubGo server..."
	./$(BUILD_DIR)/$(BINARY_NAME)

## Run the server in development mode (with hot reload if available)
dev:
	@echo "Starting development server..."
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "Air not found. Install with: go install github.com/cosmtrek/air@latest"; \
		echo "Running without hot reload..."; \
		$(GO) run ./cmd/server; \
	fi

## Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f $(COVERAGE_FILE)
	$(GO) clean -cache -testcache -modcache

# Testing targets
## Run all tests
test: test-unit test-integration

### Run unit tests with coverage
test-unit:
	@echo "Running unit tests..."
	$(GO) test -v -race -coverprofile=$(COVERAGE_FILE) -covermode=atomic \
		-timeout=$(TEST_TIMEOUT) \
		$(shell go list ./... | grep -v /test/)

### Run integration tests (requires Redis)
test-integration:
	@echo "Running integration tests..."
	@echo "Checking for Redis..."
	@if ! command -v redis-cli >/dev/null 2>&1; then \
		echo "Error: Redis not found. Install with: brew install redis (macOS) or apt-get install redis-server (Ubuntu)"; \
		exit 1; \
	fi
	@if ! redis-cli ping >/dev/null 2>&1; then \
		echo "Starting Redis..."; \
		redis-server --daemonize yes --port 6379; \
		sleep 2; \
	fi
	$(GO) test -v -race -timeout=$(TEST_TIMEOUT) ./test/integration/...

### Run fuzzing tests
test-fuzz:
	@echo "Running fuzzing tests..."
	$(GO) test -fuzz=FuzzMessageParsing -fuzztime=30s ./test/fuzzing/...
	$(GO) test -fuzz=FuzzBase64Decode -fuzztime=30s ./test/fuzzing/...
	$(GO) test -fuzz=FuzzMessageValidation -fuzztime=30s ./test/fuzzing/...

### Run benchmark tests
test-bench:
	@echo "Running benchmark tests..."
	$(GO) test -bench=. -benchmem -timeout=$(TEST_TIMEOUT) ./test/benchmark/...

### Run load tests with k6 (requires k6)
test-load:
	@echo "Running load tests..."
	@if ! command -v k6 >/dev/null 2>&1; then \
		echo "Error: k6 not found. Install from: https://k6.io/docs/getting-started/installation/"; \
		exit 1; \
	fi
	@echo "Starting test server..."
	@./$(BUILD_DIR)/$(BINARY_NAME) &
	@SERVER_PID=$$!; \
	sleep 3; \
	echo "Running k6 load tests..."; \
	k6 run test/load/pubsub_load_test.js; \
	echo "Stopping test server..."; \
	kill $$SERVER_PID

### Run short tests (for CI/quick feedback)
test-short:
	@echo "Running short tests..."
	$(GO) test -short -race -timeout=60s ./...

### Run all tests with coverage report
test-all: test-unit test-integration test-bench
	@echo "Generating coverage report..."
	$(GO) tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "Coverage report generated: coverage.html"

### Run tests in verbose mode
test-verbose:
	@echo "Running tests in verbose mode..."
	$(GO) test -v -race -coverprofile=$(COVERAGE_FILE) ./...

# Docker targets
## Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

## Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run --rm -p 8081:8080 -p 9092:9091 \
		-e PUBSUB_REDIS_HOST=host.docker.internal \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

## Build and run with Docker Compose
docker-compose-up:
	@echo "Starting services with Docker Compose..."
	@if command -v docker-compose >/dev/null 2>&1; then \
		docker-compose up --build -d; \
	else \
		docker compose up --build -d; \
	fi

## Stop Docker Compose services
docker-compose-down:
	@echo "Stopping Docker Compose services..."
	@if command -v docker-compose >/dev/null 2>&1; then \
		docker-compose down; \
	else \
		docker compose down; \
	fi

## Run tests in Docker
docker-test:
	@echo "Running tests in Docker..."
	@if command -v docker-compose >/dev/null 2>&1; then \
		docker-compose -f test/docker-compose.test.yml up --build --abort-on-container-exit test-runner; \
	else \
		docker compose -f test/docker-compose.test.yml up --build --abort-on-container-exit test-runner; \
	fi

## Build Docker test image
docker-test-build:
	@echo "Building Docker test image..."
	docker build -f test/Dockerfile.test -t $(DOCKER_IMAGE)-test .

# Kubernetes targets
## Deploy to Kubernetes (development)
k8s-deploy-dev:
	@echo "Deploying to Kubernetes (development)..."
	kubectl apply -k deployments/kubernetes/overlays/development/

## Deploy to Kubernetes (production)
k8s-deploy-prod:
	@echo "Deploying to Kubernetes (production)..."
	kubectl apply -k deployments/kubernetes/overlays/production/

## Delete Kubernetes deployment
k8s-delete:
	@echo "Deleting Kubernetes deployment..."
	kubectl delete -k deployments/kubernetes/overlays/development/

## Get Kubernetes pod status
k8s-status:
	@echo "Getting Kubernetes status..."
	kubectl get pods,services,ingress -l app=pubsubgo

## View Kubernetes logs
k8s-logs:
	@echo "Viewing Kubernetes logs..."
	kubectl logs -l app=pubsubgo --tail=100 -f

# Database/Redis targets
## Start Redis locally
redis-start:
	@echo "Starting Redis..."
	@if command -v brew >/dev/null 2>&1; then \
		brew services start redis; \
	elif command -v systemctl >/dev/null 2>&1; then \
		sudo systemctl start redis; \
	else \
		redis-server --daemonize yes; \
	fi

## Stop Redis locally
redis-stop:
	@echo "Stopping Redis..."
	@if command -v brew >/dev/null 2>&1; then \
		brew services stop redis; \
	elif command -v systemctl >/dev/null 2>&1; then \
		sudo systemctl stop redis; \
	else \
		redis-cli shutdown; \
	fi

## Redis health check
redis-check:
	@echo "Checking Redis connection..."
	@redis-cli ping

# Monitoring targets
## Start complete monitoring stack (Prometheus, Grafana, Jaeger, AlertManager)
monitoring-up:
	@echo "Starting monitoring stack..."
	@if command -v docker-compose >/dev/null 2>&1; then \
		docker-compose -f deployments/docker/monitoring.yml up -d; \
	else \
		docker compose -f deployments/docker/monitoring.yml up -d; \
	fi
	@echo "Monitoring stack started:"
	@echo "  - Grafana: http://localhost:3000 (admin/admin123)"
	@echo "  - Prometheus: http://localhost:9090"
	@echo "  - Jaeger: http://localhost:16686"
	@echo "  - AlertManager: http://localhost:9093"

## Stop monitoring stack
monitoring-down:
	@echo "Stopping monitoring stack..."
	@if command -v docker-compose >/dev/null 2>&1; then \
		docker-compose -f deployments/docker/monitoring.yml down; \
	else \
		docker compose -f deployments/docker/monitoring.yml down; \
	fi

## View monitoring logs
monitoring-logs:
	@echo "Viewing monitoring stack logs..."
	@if command -v docker-compose >/dev/null 2>&1; then \
		docker-compose -f deployments/docker/monitoring.yml logs -f; \
	else \
		docker compose -f deployments/docker/monitoring.yml logs -f; \
	fi

## Restart monitoring stack
monitoring-restart:
	@echo "Restarting monitoring stack..."
	@if command -v docker-compose >/dev/null 2>&1; then \
		docker-compose -f deployments/docker/monitoring.yml restart; \
	else \
		docker compose -f deployments/docker/monitoring.yml restart; \
	fi

## Show monitoring stack status
monitoring-status:
	@echo "Monitoring stack status:"
	@if command -v docker-compose >/dev/null 2>&1; then \
		docker-compose -f deployments/docker/monitoring.yml ps; \
	else \
		docker compose -f deployments/docker/monitoring.yml ps; \
	fi

# Security and quality targets
## Run security scan (requires gosec)
security-scan:
	@echo "Running security scan..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "Warning: gosec not found. Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"; \
	fi

## Check for vulnerabilities (requires govulncheck)
vuln-check:
	@echo "Checking for vulnerabilities..."
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
	else \
		echo "Warning: govulncheck not found. Install with: go install golang.org/x/vuln/cmd/govulncheck@latest"; \
	fi

# Release targets
## Create a new release
release: clean lint test-all build
	@echo "Creating release $(VERSION)..."
	@mkdir -p releases/$(VERSION)
	@cp $(BUILD_DIR)/* releases/$(VERSION)/
	@echo "Release $(VERSION) created in releases/$(VERSION)/"

## Show version information
version:
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Go Version: $(shell go version)"

# Documentation targets
## Generate documentation (if available)
docs:
	@echo "Generating documentation..."
	@if command -v godoc >/dev/null 2>&1; then \
		echo "Starting godoc server at http://localhost:6060"; \
		godoc -http=:6060; \
	else \
		echo "godoc not found. Install with: go install golang.org/x/tools/cmd/godoc@latest"; \
	fi

# Quick commands for common workflows
## Quick development setup (install deps, format, build)
setup: deps fmt build
	@echo "Development setup complete!"

## Quick CI check (format, lint, test)
ci: fmt lint test-all
	@echo "CI checks complete!"

## Pre-commit checks
pre-commit: fmt lint test-short
	@echo "Pre-commit checks complete!"

# Platform-specific builds
## Build for Linux
build-linux:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 $(MAKE) build

## Build for Windows
build-windows:
	@echo "Building for Windows..."
	GOOS=windows GOARCH=amd64 $(MAKE) build

## Build for macOS
build-darwin:
	@echo "Building for macOS..."
	GOOS=darwin GOARCH=amd64 $(MAKE) build

## Build for all platforms
build-all: build-linux build-windows build-darwin
	@echo "Multi-platform build complete!"