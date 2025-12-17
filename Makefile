.PHONY: build build-linux test test-e2e clean docker-test help

# Binary name
BINARY := slacker
BINARY_LINUX := slacker-linux

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod

# Build flags
LDFLAGS := -s -w

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build for current OS
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BINARY) .

build-linux: ## Build for Linux (Docker/remote servers)
	GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(BINARY_LINUX) .

test: ## Run unit tests
	$(GOTEST) -v ./...

test-e2e: ## Run e2e tests in Docker (builds inside container)
	docker build -t slacker-e2e -f Dockerfile.test .
	docker run --rm slacker-e2e

docker-test: build-linux ## Quick Docker test with basic manifest
	docker run --rm \
		-v "$(PWD)/$(BINARY_LINUX):/usr/local/bin/slacker" \
		-v "$(PWD)/e2e/basic.yaml:/tmp/manifest.yaml" \
		ubuntu:22.04 \
		/bin/bash -c "slacker local -c /tmp/manifest.yaml"

docker-test-idempotent: build-linux ## Test idempotency (runs twice)
	docker run --rm \
		-v "$(PWD)/$(BINARY_LINUX):/usr/local/bin/slacker" \
		-v "$(PWD)/e2e/basic.yaml:/tmp/manifest.yaml" \
		ubuntu:22.04 \
		/bin/bash -c "slacker local -c /tmp/manifest.yaml && echo '--- Second run ---' && slacker local -c /tmp/manifest.yaml"

clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -f $(BINARY) $(BINARY_LINUX)

deps: ## Download dependencies
	$(GOMOD) download
	$(GOMOD) tidy

lint: ## Run linter (requires golangci-lint)
	golangci-lint run ./...

.DEFAULT_GOAL := help

