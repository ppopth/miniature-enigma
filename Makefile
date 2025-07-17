# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=simple
BINARY_UNIX=$(BINARY_NAME)_unix

# Test parameters
TEST_PACKAGES=./...
COVERAGE_OUT=coverage.out

.PHONY: all build clean test coverage deps lint proto example help

all: deps lint test build ## Run all checks and build

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

deps: ## Download dependencies
	$(GOMOD) download
	$(GOMOD) verify

build: ## Build the project
	$(GOBUILD) -v ./...

clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)
	rm -f $(COVERAGE_OUT)

test: ## Run tests
	$(GOTEST) -v -race ./...

test-short: ## Run tests without race detection
	$(GOTEST) -v ./...

coverage: ## Run tests with coverage
	$(GOTEST) -race -coverprofile=$(COVERAGE_OUT) -covermode=atomic $(TEST_PACKAGES)
	$(GOCMD) tool cover -html=$(COVERAGE_OUT)

lint: ## Run linter
	golangci-lint run --timeout=5m

proto: ## Generate protobuf files
	cd pb && $(MAKE)

proto-clean: ## Clean protobuf generated files
	cd pb && $(MAKE) clean

example: ## Build example application
	cd examples/simple && $(GOBUILD) -o $(BINARY_NAME) -v .

example-linux: ## Cross compile example for Linux
	cd examples/simple && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_UNIX) -v .

ci: deps lint test build example ## Run CI pipeline locally

# Install development tools
install-tools: ## Install development tools
	$(GOGET) github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GOGET) github.com/gogo/protobuf/protoc-gen-gofast@latest