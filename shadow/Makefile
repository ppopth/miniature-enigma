# Master Makefile for eth-ec-broadcast Shadow simulations

.PHONY: all help clean floodsub rlnc rs test-all clean-all

# Default parameters
NODE_COUNT ?= 10
MSG_SIZE ?= 64
MSG_COUNT ?= 5

# Run floodsub simulation
floodsub:
	@echo "Running FloodSub simulation..."
	cd floodsub && make all NODE_COUNT=$(NODE_COUNT) MSG_SIZE=$(MSG_SIZE) MSG_COUNT=$(MSG_COUNT)

# Run RLNC simulation
rlnc:
	@echo "Running RLNC simulation..."
	cd rlnc && make all NODE_COUNT=$(NODE_COUNT) MSG_SIZE=$(MSG_SIZE) MSG_COUNT=$(MSG_COUNT)

# Run Reed-Solomon simulation
rs:
	@echo "Running Reed-Solomon simulation..."
	cd rs && make all NODE_COUNT=$(NODE_COUNT) MSG_SIZE=$(MSG_SIZE) MSG_COUNT=$(MSG_COUNT)

# Test all protocol results
test-all:
	@echo "Testing all protocol simulation results..."
	cd floodsub && make test NODE_COUNT=$(NODE_COUNT) MSG_COUNT=$(MSG_COUNT) || echo "FloodSub test failed"
	cd rlnc && make test NODE_COUNT=$(NODE_COUNT) MSG_COUNT=$(MSG_COUNT) || echo "RLNC test failed"
	cd rs && make test NODE_COUNT=$(NODE_COUNT) MSG_COUNT=$(MSG_COUNT) || echo "Reed-Solomon test failed"

# Clean all protocol directories
clean-all:
	@echo "Cleaning all protocol directories..."
	cd floodsub && make clean
	cd rlnc && make clean
	cd rs && make clean

# Run all protocols (default)
all: floodsub rlnc rs

# Clean (alias for clean-all)
clean: clean-all

# Help target
help:
	@echo "eth-ec-broadcast Shadow Simulation Suite"
	@echo "======================================="
	@echo ""
	@echo "This master Makefile coordinates simulations for all three protocols:"
	@echo "- FloodSub: Basic message flooding protocol"
	@echo "- RLNC: Random Linear Network Coding with erasure coding"
	@echo "- Reed-Solomon: Reed-Solomon erasure coding"
	@echo ""
	@echo "Targets:"
	@echo "  floodsub        - Run FloodSub simulation"
	@echo "  rlnc            - Run RLNC simulation"
	@echo "  rs              - Run Reed-Solomon simulation"
	@echo "  all             - Run all three protocol simulations"
	@echo "  test-all        - Test results from all protocols"
	@echo "  clean-all       - Clean all protocol directories"
	@echo "  clean           - Alias for clean-all"
	@echo "  help            - Show this help message"
	@echo ""
	@echo "Configuration variables:"
	@echo "  NODE_COUNT      - Number of nodes (default: $(NODE_COUNT))"
	@echo "  MSG_SIZE        - Message size in bytes (default: $(MSG_SIZE))"
	@echo "  MSG_COUNT       - Number of messages (default: $(MSG_COUNT))"
	@echo ""
	@echo "Examples:"
	@echo "  make all                                    # Run all protocols with defaults"
	@echo "  make floodsub NODE_COUNT=20 MSG_SIZE=128   # Run FloodSub with custom parameters"
	@echo "  make rlnc NODE_COUNT=15                    # Run RLNC with 15 nodes"
	@echo "  make test-all                              # Test all existing results"
	@echo ""
	@echo "Individual protocol help:"
	@echo "  cd floodsub && make help                   # FloodSub-specific help"
	@echo "  cd rlnc && make help                       # RLNC-specific help"
	@echo "  cd rs && make help                         # Reed-Solomon-specific help"
