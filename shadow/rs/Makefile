# Makefile for eth-ec-broadcast Reed-Solomon Shadow simulation

.PHONY: all build clean run-sim test help check-deps

# Default parameters
NODE_COUNT ?= 10
MSG_SIZE ?= 64
MSG_COUNT ?= 5

# Check dependencies
check-deps:
	@echo "Checking dependencies..."
	@which shadow > /dev/null || (echo "Error: Shadow simulator not found. Install from https://shadow.github.io/" && exit 1)
	@which python3 > /dev/null || (echo "Error: Python 3 not found" && exit 1)
	@python3 -c "import networkx, yaml" 2>/dev/null || (echo "Error: Install required packages: pip3 install networkx pyyaml" && exit 1)
	@echo "Dependencies OK"

# Build the Reed-Solomon simulation binary
build: check-deps
	@echo "Building Reed-Solomon simulation binary..."
	cd ../.. && go build -o shadow/rs/rs-sim ./shadow/rs
	@test -f rs-sim || (echo "Build failed" && exit 1)
	@echo "Build successful"

# Generate network graph and Shadow configuration
generate-config: check-deps
	@echo "Generating Shadow network configuration for $(NODE_COUNT) nodes (Reed-Solomon)..."
	python3 ../network_graph.py $(NODE_COUNT) $(MSG_SIZE) $(MSG_COUNT) rs
	@test -f shadow-rs.yaml && test -f graph.gml || (echo "Config generation failed" && exit 1)
	@echo "Configuration generated"

# Run the complete Shadow simulation
run-sim: build generate-config
	@echo "Starting Reed-Solomon Shadow simulation ($(NODE_COUNT) nodes, $(MSG_COUNT) messages of $(MSG_SIZE) bytes)..."
	shadow shadow-rs.yaml
	@echo "Reed-Solomon simulation completed"

# Test simulation results by reading Shadow logs
test:
	@echo "Testing Reed-Solomon simulation results..."
	python3 ../test_results.py $(NODE_COUNT) $(MSG_COUNT) rs

# Clean build artifacts and simulation results
clean:
	@echo "Cleaning up..."
	rm -f rs-sim
	rm -f shadow-rs.yaml
	rm -f graph.gml
	rm -f node-*-results.txt
	rm -rf shadow.data/
	@echo "Cleanup completed"

# Complete workflow: build, run, and test
all: run-sim test

# Help target
help:
	@echo "eth-ec-broadcast Reed-Solomon Shadow Simulation"
	@echo "=============================================="
	@echo ""
	@echo "Targets:"
	@echo "  check-deps      - Check required dependencies"
	@echo "  build           - Build the Reed-Solomon simulation binary"
	@echo "  generate-config - Generate Shadow configuration"
	@echo "  run-sim         - Run complete Shadow simulation"
	@echo "  test            - Test simulation results from Shadow logs"
	@echo "  all             - Run simulation and test results"
	@echo "  clean           - Clean up build artifacts and results"
	@echo "  help            - Show this help message"
	@echo ""
	@echo "Configuration variables:"
	@echo "  NODE_COUNT      - Number of nodes (default: $(NODE_COUNT))"
	@echo "  MSG_SIZE        - Message size in bytes (default: $(MSG_SIZE))"
	@echo "  MSG_COUNT       - Number of messages (default: $(MSG_COUNT))"
	@echo ""
	@echo "Examples:"
	@echo "  make all                                    # Run with defaults"
	@echo "  make run-sim NODE_COUNT=20 MSG_SIZE=128    # Custom parameters"
	@echo "  make test                                  # Test existing results"