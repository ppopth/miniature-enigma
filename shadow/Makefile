# Master Makefile for eth-ec-broadcast Shadow simulations

.PHONY: all help clean floodsub rlnc rs test-all clean-all all-topologies

# Default parameters
NODE_COUNT ?= 10
MSG_SIZE ?= 64
MSG_COUNT ?= 5
NUM_CHUNKS ?= 8
PROGRESS ?= false

# Run floodsub simulation
floodsub:
	@echo "Running FloodSub simulation..."
	cd floodsub && make all NODE_COUNT=$(NODE_COUNT) MSG_SIZE=$(MSG_SIZE) MSG_COUNT=$(MSG_COUNT) PROGRESS=$(PROGRESS)

# Run RLNC simulation
rlnc:
	@echo "Running RLNC simulation..."
	cd rlnc && make all NODE_COUNT=$(NODE_COUNT) MSG_SIZE=$(MSG_SIZE) MSG_COUNT=$(MSG_COUNT) NUM_CHUNKS=$(NUM_CHUNKS) PROGRESS=$(PROGRESS)

# Run Reed-Solomon simulation
rs:
	@echo "Running Reed-Solomon simulation..."
	cd rs && make all NODE_COUNT=$(NODE_COUNT) MSG_SIZE=$(MSG_SIZE) MSG_COUNT=$(MSG_COUNT) NUM_CHUNKS=$(NUM_CHUNKS) PROGRESS=$(PROGRESS)

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

# Test all topology types with all protocols (comprehensive matrix)
all-topologies:
	@echo "Testing ALL topology types with ALL protocols..."
	@echo "Building topology generator..."
	cd topology/gen && go build -o topology-gen main.go
	@echo "Generating test topology files..."
	cd topology/gen && \
		./topology-gen -type linear -nodes 5 -output ../topology-5.json && \
		./topology-gen -type ring -nodes 6 -output ../topology-6.json && \
		./topology-gen -type mesh -nodes 4 -output ../topology-4.json && \
		./topology-gen -type tree -nodes 8 -branching 2 -output ../topology-8.json && \
		./topology-gen -type small-world -nodes 9 -k 2 -rewire 0.3 -output ../topology-9.json && \
		./topology-gen -type random-regular -nodes 7 -degree 3 -output ../topology-7.json
	@echo ""
	@echo "=== Testing Linear topology (5 nodes) ==="
	@echo "Linear + FloodSub..."
	$(MAKE) clean-all && $(MAKE) floodsub NODE_COUNT=5 MSG_COUNT=1
	@echo "Linear + RLNC..."
	$(MAKE) clean-all && $(MAKE) rlnc NODE_COUNT=5 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS)
	@echo "Linear + Reed-Solomon..."
	$(MAKE) clean-all && $(MAKE) rs NODE_COUNT=5 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS)
	@echo ""
	@echo "=== Testing Ring topology (6 nodes) ==="
	@echo "Ring + FloodSub..."
	$(MAKE) clean-all && $(MAKE) floodsub NODE_COUNT=6 MSG_COUNT=1
	@echo "Ring + RLNC..."
	$(MAKE) clean-all && $(MAKE) rlnc NODE_COUNT=6 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS)
	@echo "Ring + Reed-Solomon..."
	$(MAKE) clean-all && $(MAKE) rs NODE_COUNT=6 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS)
	@echo ""
	@echo "=== Testing Mesh topology (4 nodes) ==="
	@echo "Mesh + FloodSub..."
	$(MAKE) clean-all && $(MAKE) floodsub NODE_COUNT=4 MSG_COUNT=1
	@echo "Mesh + RLNC..."
	$(MAKE) clean-all && $(MAKE) rlnc NODE_COUNT=4 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS)
	@echo "Mesh + Reed-Solomon..."
	$(MAKE) clean-all && $(MAKE) rs NODE_COUNT=4 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS)
	@echo ""
	@echo "=== Testing Tree topology (8 nodes) ==="
	@echo "Tree + FloodSub..."
	$(MAKE) clean-all && $(MAKE) floodsub NODE_COUNT=8 MSG_COUNT=1
	@echo "Tree + RLNC..."
	$(MAKE) clean-all && $(MAKE) rlnc NODE_COUNT=8 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS)
	@echo "Tree + Reed-Solomon..."
	$(MAKE) clean-all && $(MAKE) rs NODE_COUNT=8 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS)
	@echo ""
	@echo "=== Testing Small-World topology (9 nodes) ==="
	@echo "Small-World + FloodSub..."
	$(MAKE) clean-all && $(MAKE) floodsub NODE_COUNT=9 MSG_COUNT=1
	@echo "Small-World + RLNC..."
	$(MAKE) clean-all && $(MAKE) rlnc NODE_COUNT=9 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS)
	@echo "Small-World + Reed-Solomon..."
	$(MAKE) clean-all && $(MAKE) rs NODE_COUNT=9 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS)
	@echo ""
	@echo "=== Testing Random-Regular topology (7 nodes) ==="
	@echo "Random-Regular + FloodSub..."
	$(MAKE) clean-all && $(MAKE) floodsub NODE_COUNT=7 MSG_COUNT=1
	@echo "Random-Regular + RLNC..."
	$(MAKE) clean-all && $(MAKE) rlnc NODE_COUNT=7 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS)
	@echo "Random-Regular + Reed-Solomon..."
	$(MAKE) clean-all && $(MAKE) rs NODE_COUNT=7 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS)
	@echo ""
	@echo "🎉 ALL topology/protocol combinations tested successfully!"
	@echo "   - 6 topology types × 3 protocols = 18 total test combinations"

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
	@echo "  all-topologies  - Test all topology types across protocols"
	@echo "  clean-all       - Clean all protocol directories"
	@echo "  clean           - Alias for clean-all"
	@echo "  help            - Show this help message"
	@echo ""
	@echo "Configuration variables:"
	@echo "  NODE_COUNT      - Number of nodes (default: $(NODE_COUNT))"
	@echo "  MSG_SIZE        - Message size in bytes (default: $(MSG_SIZE))"
	@echo "  MSG_COUNT       - Number of messages (default: $(MSG_COUNT))"
	@echo "  NUM_CHUNKS      - Number of chunks for RLNC and RS (default: $(NUM_CHUNKS))"
	@echo "  PROGRESS        - Show Shadow progress bar (default: $(PROGRESS))"
	@echo ""
	@echo "Examples:"
	@echo "  make all                                    # Run all protocols with defaults"
	@echo "  make floodsub NODE_COUNT=20 MSG_SIZE=128   # Run FloodSub with custom parameters"
	@echo "  make rlnc NODE_COUNT=15                    # Run RLNC with 15 nodes"
	@echo "  make rlnc MSG_SIZE=256 NUM_CHUNKS=32       # Run RLNC with 32 chunks"
	@echo "  make rs MSG_SIZE=64 NUM_CHUNKS=8           # Run Reed-Solomon with 8 chunks"
	@echo "  make floodsub PROGRESS=true                # Run FloodSub with progress bar"
	@echo "  make test-all                              # Test all existing results"
	@echo ""
	@echo "Individual protocol help:"
	@echo "  cd floodsub && make help                   # FloodSub-specific help"
	@echo "  cd rlnc && make help                       # RLNC-specific help"
	@echo "  cd rs && make help                         # Reed-Solomon-specific help"
