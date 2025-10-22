# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Test parameters
TEST_PACKAGES=./...
COVERAGE_OUT=coverage.out

.PHONY: all build clean test coverage deps proto example help bench bench-rlnc bench-rs bench-field test-examples

all: deps test build ## Run all checks and build

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

deps: ## Download dependencies
	$(GOMOD) download
	$(GOMOD) verify

build: ## Build the project
	$(GOBUILD) -v ./...

clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -f $(COVERAGE_OUT)
	rm -f examples/rlnc-network/rlnc-network
	rm -f examples/rlnc-network/rlnc-network_unix
	rm -f examples/rs-network/rs-network
	rm -f examples/rs-network/rs-network_unix

test: ## Run tests
	$(GOTEST) -v -race ./...

test-short: ## Run tests without race detection
	$(GOTEST) -v ./...

coverage: ## Run tests with coverage
	$(GOTEST) -race -coverprofile=$(COVERAGE_OUT) -covermode=atomic $(TEST_PACKAGES)
	$(GOCMD) tool cover -html=$(COVERAGE_OUT)

bench: ## Run benchmarks for all packages
	$(GOTEST) -bench=. -benchmem ./...

bench-rlnc: ## Run RLNC encoder benchmarks
	$(GOTEST) -bench=. -benchmem ./ec/encode/rlnc

bench-rs: ## Run Reed-Solomon encoder benchmarks
	$(GOTEST) -bench=. -benchmem ./ec/encode/rs

bench-field: ## Run field arithmetic benchmarks
	$(GOTEST) -bench=. -benchmem ./ec/field

test-examples: ## Test example applications
	cd examples/rlnc-network && chmod +x test.sh && ./test.sh
	cd examples/rs-network && chmod +x test.sh && ./test.sh

proto: ## Generate protobuf files
	cd pb && $(MAKE)

proto-clean: ## Clean protobuf generated files
	cd pb && $(MAKE) clean

example: ## Build example applications
	cd examples/rlnc-network && $(GOBUILD) -o rlnc-network -v .
	cd examples/rs-network && $(GOBUILD) -o rs-network -v .

example-linux: ## Cross compile examples for Linux
	cd examples/rlnc-network && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o rlnc-network_unix -v .
	cd examples/rs-network && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o rs-network_unix -v .


# Install development tools
install-tools: ## Install development tools
	$(GOGET) github.com/gogo/protobuf/protoc-gen-gofast@latest