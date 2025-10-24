# Makefile for eth-ec-broadcast GossipSub Shadow simulation

.PHONY: all build clean run-sim test help check-deps

# Default parameters
NODE_COUNT ?= 10
MSG_SIZE ?= 256
MSG_COUNT ?= 5
PROGRESS ?= false
LOG_LEVEL ?= info

# Check dependencies
check-deps:
	@echo "Checking dependencies..."
	@which shadow > /dev/null || (echo "Error: Shadow simulator not found. Install from https://shadow.github.io/" && exit 1)
	@which python3 > /dev/null || (echo "Error: Python 3 not found" && exit 1)
	@python3 -c "import networkx, yaml" 2>/dev/null || (echo "Error: Install required packages: pip3 install networkx pyyaml" && exit 1)
	@echo "Dependencies OK"

# Build the gossipsub simulation binary
build: check-deps
	@echo "Building GossipSub simulation binary..."
	cd ../.. && go build -o shadow/gossipsub/gossipsub-sim ./shadow/gossipsub
	@test -f gossipsub-sim || (echo "Build failed" && exit 1)
	@echo "Build successful"

# Generate network graph and Shadow configuration
generate-config: check-deps
	@echo "Generating Shadow network configuration for $(NODE_COUNT) nodes (gossipsub)..."
	python3 ../network_graph.py $(NODE_COUNT) $(MSG_SIZE) $(MSG_COUNT) gossipsub "--log-level $(LOG_LEVEL)"
	@test -f shadow-gossipsub.yaml && test -f graph.gml || (echo "Config generation failed" && exit 1)
	@echo "Configuration generated"

# Run the complete Shadow simulation
run-sim: build generate-config
	@echo "Starting GossipSub Shadow simulation ($(NODE_COUNT) nodes, $(MSG_COUNT) messages of $(MSG_SIZE) bytes)..."
	@rm -rf shadow.data/
	shadow --progress $(PROGRESS) shadow-gossipsub.yaml
	@echo "GossipSub simulation completed"

# Test simulation results by reading Shadow logs
test:
	@echo "Testing GossipSub simulation results..."
	python3 ../test_results.py $(NODE_COUNT) $(MSG_COUNT) gossipsub

# Clean build artifacts and simulation results
clean:
	@echo "Cleaning up..."
	rm -f gossipsub-sim
	rm -f shadow-gossipsub.yaml
	rm -f graph.gml
	rm -f node-*-results.txt
	rm -rf shadow.data/
	@echo "Cleanup completed"

# Complete workflow: build, run, and test
all: run-sim test

# Help target
help:
	@echo "eth-ec-broadcast GossipSub Shadow Simulation"
	@echo "==========================================="
	@echo ""
	@echo "Targets:"
	@echo "  check-deps      - Check required dependencies"
	@echo "  build           - Build the GossipSub simulation binary"
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
	@echo "  PROGRESS        - Show Shadow progress bar (default: $(PROGRESS))"
	@echo ""
	@echo "Examples:"
	@echo "  make all                                    # Run with defaults"
	@echo "  make run-sim NODE_COUNT=20 MSG_SIZE=512    # Custom parameters"
	@echo "  make run-sim PROGRESS=true                 # Run with progress bar"
	@echo "  make test                                  # Test existing results"
