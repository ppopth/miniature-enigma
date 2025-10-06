# Makefile for eth-ec-broadcast FloodSub Shadow simulation

.PHONY: all build clean run-sim test help check-deps

# Default parameters
NODE_COUNT ?= 10
MSG_SIZE ?= 64
MSG_COUNT ?= 5
PROGRESS ?= false

# Check dependencies
check-deps:
	@echo "Checking dependencies..."
	@which shadow > /dev/null || (echo "Error: Shadow simulator not found. Install from https://shadow.github.io/" && exit 1)
	@which python3 > /dev/null || (echo "Error: Python 3 not found" && exit 1)
	@python3 -c "import networkx, yaml" 2>/dev/null || (echo "Error: Install required packages: pip3 install networkx pyyaml" && exit 1)
	@echo "Dependencies OK"

# Build the floodsub simulation binary
build: check-deps
	@echo "Building floodsub simulation binary..."
	cd ../.. && go build -o shadow/floodsub/floodsub-sim ./shadow/floodsub
	@test -f floodsub-sim || (echo "Build failed" && exit 1)
	@echo "Build successful"

# Generate network graph and Shadow configuration
generate-config: check-deps
	@echo "Generating Shadow network configuration for $(NODE_COUNT) nodes (floodsub)..."
	python3 ../network_graph.py $(NODE_COUNT) $(MSG_SIZE) $(MSG_COUNT) floodsub
	@test -f shadow-floodsub.yaml && test -f graph.gml || (echo "Config generation failed" && exit 1)
	@echo "Configuration generated"

# Generate network graph and Shadow configuration for streams
generate-config-streams: check-deps
	@echo "Generating Shadow network configuration for $(NODE_COUNT) nodes (floodsub with streams)..."
	python3 ../network_graph.py $(NODE_COUNT) $(MSG_SIZE) $(MSG_COUNT) floodsub --use-streams
	@test -f shadow-floodsub.yaml && test -f graph.gml || (echo "Config generation failed" && exit 1)
	@echo "Configuration generated (with QUIC streams)"

# Run the complete Shadow simulation
run-sim: build generate-config
	@echo "Starting FloodSub Shadow simulation ($(NODE_COUNT) nodes, $(MSG_COUNT) messages of $(MSG_SIZE) bytes)..."
	@rm -rf shadow.data/
	shadow --progress $(PROGRESS) shadow-floodsub.yaml
	@echo "FloodSub simulation completed"

# Test simulation results by reading Shadow logs
test:
	@echo "Testing FloodSub simulation results..."
	python3 ../test_results.py $(NODE_COUNT) $(MSG_COUNT) floodsub

# Clean build artifacts and simulation results
clean:
	@echo "Cleaning up..."
	rm -f floodsub-sim
	rm -f shadow-floodsub.yaml
	rm -f graph.gml
	rm -f node-*-results.txt
	rm -rf shadow.data/
	@echo "Cleanup completed"

# Run the complete Shadow simulation with QUIC streams
run-sim-streams: build generate-config-streams
	@echo "Starting FloodSub Shadow simulation with QUIC streams ($(NODE_COUNT) nodes, $(MSG_COUNT) messages of $(MSG_SIZE) bytes)..."
	@rm -rf shadow.data/
	shadow --progress $(PROGRESS) shadow-floodsub.yaml
	@echo "FloodSub simulation with streams completed"

# Complete workflow: build, run, and test
all: run-sim test

# Complete workflow with QUIC streams: build, run, and test
all-streams: run-sim-streams test

# Help target
help:
	@echo "eth-ec-broadcast FloodSub Shadow Simulation"
	@echo "=========================================="
	@echo ""
	@echo "Targets:"
	@echo "  check-deps              - Check required dependencies"
	@echo "  build                   - Build the floodsub simulation binary"
	@echo "  generate-config         - Generate Shadow configuration (QUIC datagrams)"
	@echo "  generate-config-streams - Generate Shadow configuration (QUIC streams)"
	@echo "  run-sim                 - Run complete Shadow simulation (QUIC datagrams)"
	@echo "  run-sim-streams         - Run complete Shadow simulation (QUIC streams)"
	@echo "  test                    - Test simulation results from Shadow logs"
	@echo "  all                     - Run simulation with datagrams and test results"
	@echo "  all-streams             - Run simulation with streams and test results"
	@echo "  clean                   - Clean up build artifacts and results"
	@echo "  help                    - Show this help message"
	@echo ""
	@echo "Configuration variables:"
	@echo "  NODE_COUNT      - Number of nodes (default: $(NODE_COUNT))"
	@echo "  MSG_SIZE        - Message size in bytes (default: $(MSG_SIZE))"
	@echo "  MSG_COUNT       - Number of messages (default: $(MSG_COUNT))"
	@echo "  PROGRESS        - Show Shadow progress bar (default: $(PROGRESS))"
	@echo ""
	@echo "Examples:"
	@echo "  make all                                    # Run with defaults (QUIC datagrams)"
	@echo "  make all-streams                            # Run with QUIC streams"
	@echo "  make run-sim NODE_COUNT=20 MSG_SIZE=128    # Custom parameters"
	@echo "  make run-sim-streams NODE_COUNT=20         # Run with QUIC streams"
	@echo "  make run-sim PROGRESS=true                 # Run with progress bar"
	@echo "  make test                                  # Test existing results"