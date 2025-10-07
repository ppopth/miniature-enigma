# Master Makefile for eth-ec-broadcast Shadow simulations

.PHONY: all help clean floodsub floodsub-streams gossipsub rlnc rlnc-streams rs rs-streams test-all clean-all all-topologies

# Default parameters
NODE_COUNT ?= 10
MSG_SIZE ?= 64
MSG_COUNT ?= 5
NUM_CHUNKS ?= 8
PROGRESS ?= false
LOG_LEVEL ?= info

# Run floodsub simulation
floodsub:
	@echo "Running FloodSub simulation..."
	cd floodsub && make all NODE_COUNT=$(NODE_COUNT) MSG_SIZE=$(MSG_SIZE) MSG_COUNT=$(MSG_COUNT) PROGRESS=$(PROGRESS) LOG_LEVEL=$(LOG_LEVEL)

# Run floodsub simulation with QUIC streams
floodsub-streams:
	@echo "Running FloodSub simulation with QUIC streams..."
	cd floodsub && make all-streams NODE_COUNT=$(NODE_COUNT) MSG_SIZE=$(MSG_SIZE) MSG_COUNT=$(MSG_COUNT) PROGRESS=$(PROGRESS) LOG_LEVEL=$(LOG_LEVEL)

# Run gossipsub simulation
gossipsub:
	@echo "Running GossipSub simulation (libp2p)..."
	cd gossipsub && make all NODE_COUNT=$(NODE_COUNT) MSG_SIZE=$(MSG_SIZE) MSG_COUNT=$(MSG_COUNT) PROGRESS=$(PROGRESS) LOG_LEVEL=$(LOG_LEVEL)

# Run RLNC simulation
rlnc:
	@echo "Running RLNC simulation..."
	cd rlnc && make all NODE_COUNT=$(NODE_COUNT) MSG_SIZE=$(MSG_SIZE) MSG_COUNT=$(MSG_COUNT) NUM_CHUNKS=$(NUM_CHUNKS) PROGRESS=$(PROGRESS) LOG_LEVEL=$(LOG_LEVEL)

# Run RLNC simulation with QUIC streams
rlnc-streams:
	@echo "Running RLNC simulation with QUIC streams..."
	cd rlnc && make all-streams NODE_COUNT=$(NODE_COUNT) MSG_SIZE=$(MSG_SIZE) MSG_COUNT=$(MSG_COUNT) NUM_CHUNKS=$(NUM_CHUNKS) PROGRESS=$(PROGRESS) LOG_LEVEL=$(LOG_LEVEL)

# Run Reed-Solomon simulation
rs:
	@echo "Running Reed-Solomon simulation..."
	cd rs && make all NODE_COUNT=$(NODE_COUNT) MSG_SIZE=$(MSG_SIZE) MSG_COUNT=$(MSG_COUNT) NUM_CHUNKS=$(NUM_CHUNKS) PROGRESS=$(PROGRESS) LOG_LEVEL=$(LOG_LEVEL)

# Run Reed-Solomon simulation with QUIC streams
rs-streams:
	@echo "Running Reed-Solomon simulation with QUIC streams..."
	cd rs && make all-streams NODE_COUNT=$(NODE_COUNT) MSG_SIZE=$(MSG_SIZE) MSG_COUNT=$(MSG_COUNT) NUM_CHUNKS=$(NUM_CHUNKS) PROGRESS=$(PROGRESS) LOG_LEVEL=$(LOG_LEVEL)

# Test all protocol results
test-all:
	@echo "Testing all protocol simulation results..."
	cd floodsub && make test NODE_COUNT=$(NODE_COUNT) MSG_COUNT=$(MSG_COUNT) || echo "FloodSub test failed"
	cd gossipsub && make test NODE_COUNT=$(NODE_COUNT) MSG_COUNT=$(MSG_COUNT) || echo "GossipSub test failed"
	cd rlnc && make test NODE_COUNT=$(NODE_COUNT) MSG_COUNT=$(MSG_COUNT) || echo "RLNC test failed"
	cd rs && make test NODE_COUNT=$(NODE_COUNT) MSG_COUNT=$(MSG_COUNT) || echo "Reed-Solomon test failed"

# Clean all protocol directories
clean-all:
	@echo "Cleaning all protocol directories..."
	cd floodsub && make clean
	cd gossipsub && make clean
	cd rlnc && make clean
	cd rs && make clean

# Run all protocols (default)
all: floodsub floodsub-streams gossipsub rlnc rlnc-streams rs rs-streams

# Clean (alias for clean-all)
clean: clean-all

# Helper function to retry a test up to 5 times
define retry_test
	@for attempt in 1 2 3 4 5; do \
		echo "[Attempt $$attempt/5] $(1)"; \
		if $(MAKE) clean-all && $(MAKE) $(2); then \
			echo "✓ $(1) passed"; \
			break; \
		else \
			if [ $$attempt -eq 5 ]; then \
				echo "✗ $(1) failed after 5 attempts"; \
				exit 1; \
			fi; \
			echo "Retrying in 2 seconds..."; \
			sleep 2; \
		fi; \
	done
endef

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
	$(call retry_test,Linear + FloodSub (datagrams),floodsub NODE_COUNT=5 MSG_COUNT=1)
	$(call retry_test,Linear + FloodSub (streams),floodsub-streams NODE_COUNT=5 MSG_COUNT=1)
	$(call retry_test,Linear + GossipSub,gossipsub NODE_COUNT=5 MSG_COUNT=1)
	$(call retry_test,Linear + RLNC (datagrams),rlnc NODE_COUNT=5 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	$(call retry_test,Linear + RLNC (streams),rlnc-streams NODE_COUNT=5 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	$(call retry_test,Linear + RS (datagrams),rs NODE_COUNT=5 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	$(call retry_test,Linear + RS (streams),rs-streams NODE_COUNT=5 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	@echo ""
	@echo "=== Testing Ring topology (6 nodes) ==="
	$(call retry_test,Ring + FloodSub (datagrams),floodsub NODE_COUNT=6 MSG_COUNT=1)
	$(call retry_test,Ring + FloodSub (streams),floodsub-streams NODE_COUNT=6 MSG_COUNT=1)
	$(call retry_test,Ring + GossipSub,gossipsub NODE_COUNT=6 MSG_COUNT=1)
	$(call retry_test,Ring + RLNC (datagrams),rlnc NODE_COUNT=6 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	$(call retry_test,Ring + RLNC (streams),rlnc-streams NODE_COUNT=6 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	$(call retry_test,Ring + RS (datagrams),rs NODE_COUNT=6 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	$(call retry_test,Ring + RS (streams),rs-streams NODE_COUNT=6 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	@echo ""
	@echo "=== Testing Mesh topology (4 nodes) ==="
	$(call retry_test,Mesh + FloodSub (datagrams),floodsub NODE_COUNT=4 MSG_COUNT=1)
	$(call retry_test,Mesh + FloodSub (streams),floodsub-streams NODE_COUNT=4 MSG_COUNT=1)
	$(call retry_test,Mesh + GossipSub,gossipsub NODE_COUNT=4 MSG_COUNT=1)
	$(call retry_test,Mesh + RLNC (datagrams),rlnc NODE_COUNT=4 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	$(call retry_test,Mesh + RLNC (streams),rlnc-streams NODE_COUNT=4 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	$(call retry_test,Mesh + RS (datagrams),rs NODE_COUNT=4 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	$(call retry_test,Mesh + RS (streams),rs-streams NODE_COUNT=4 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	@echo ""
	@echo "=== Testing Tree topology (8 nodes) ==="
	$(call retry_test,Tree + FloodSub (datagrams),floodsub NODE_COUNT=8 MSG_COUNT=1)
	$(call retry_test,Tree + FloodSub (streams),floodsub-streams NODE_COUNT=8 MSG_COUNT=1)
	$(call retry_test,Tree + GossipSub,gossipsub NODE_COUNT=8 MSG_COUNT=1)
	$(call retry_test,Tree + RLNC (datagrams),rlnc NODE_COUNT=8 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	$(call retry_test,Tree + RLNC (streams),rlnc-streams NODE_COUNT=8 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	$(call retry_test,Tree + RS (datagrams),rs NODE_COUNT=8 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	$(call retry_test,Tree + RS (streams),rs-streams NODE_COUNT=8 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	@echo ""
	@echo "=== Testing Small-World topology (9 nodes) ==="
	$(call retry_test,Small-World + FloodSub (datagrams),floodsub NODE_COUNT=9 MSG_COUNT=1)
	$(call retry_test,Small-World + FloodSub (streams),floodsub-streams NODE_COUNT=9 MSG_COUNT=1)
	$(call retry_test,Small-World + GossipSub,gossipsub NODE_COUNT=9 MSG_COUNT=1)
	$(call retry_test,Small-World + RLNC (datagrams),rlnc NODE_COUNT=9 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	$(call retry_test,Small-World + RLNC (streams),rlnc-streams NODE_COUNT=9 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	$(call retry_test,Small-World + RS (datagrams),rs NODE_COUNT=9 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	$(call retry_test,Small-World + RS (streams),rs-streams NODE_COUNT=9 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	@echo ""
	@echo "=== Testing Random-Regular topology (7 nodes) ==="
	$(call retry_test,Random-Regular + FloodSub (datagrams),floodsub NODE_COUNT=7 MSG_COUNT=1)
	$(call retry_test,Random-Regular + FloodSub (streams),floodsub-streams NODE_COUNT=7 MSG_COUNT=1)
	$(call retry_test,Random-Regular + GossipSub,gossipsub NODE_COUNT=7 MSG_COUNT=1)
	$(call retry_test,Random-Regular + RLNC (datagrams),rlnc NODE_COUNT=7 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	$(call retry_test,Random-Regular + RLNC (streams),rlnc-streams NODE_COUNT=7 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	$(call retry_test,Random-Regular + RS (datagrams),rs NODE_COUNT=7 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	$(call retry_test,Random-Regular + RS (streams),rs-streams NODE_COUNT=7 MSG_COUNT=1 NUM_CHUNKS=$(NUM_CHUNKS))
	@echo ""
	@echo "🎉 ALL topology/protocol combinations tested successfully!"
	@echo "   - 6 topology types × 7 protocol variants = 42 total test combinations"
	@echo "   - Protocols: FloodSub (datagrams/streams), GossipSub, RLNC (datagrams/streams), RS (datagrams/streams)"

# Help target
help:
	@echo "eth-ec-broadcast Shadow Simulation Suite"
	@echo "======================================="
	@echo ""
	@echo "This master Makefile coordinates simulations for all protocols:"
	@echo "- FloodSub: Basic message flooding protocol"
	@echo "- GossipSub: libp2p GossipSub protocol (go-libp2p-pubsub)"
	@echo "- RLNC: Random Linear Network Coding with erasure coding"
	@echo "- Reed-Solomon: Reed-Solomon erasure coding"
	@echo ""
	@echo "Targets:"
	@echo "  floodsub        - Run FloodSub simulation (QUIC datagrams)"
	@echo "  floodsub-streams- Run FloodSub simulation (QUIC streams)"
	@echo "  gossipsub       - Run GossipSub simulation (libp2p)"
	@echo "  rlnc            - Run RLNC simulation (QUIC datagrams)"
	@echo "  rlnc-streams    - Run RLNC simulation (QUIC streams)"
	@echo "  rs              - Run Reed-Solomon simulation (QUIC datagrams)"
	@echo "  rs-streams      - Run Reed-Solomon simulation (QUIC streams)"
	@echo "  all             - Run all protocol simulations with all transport modes"
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
	@echo "  make floodsub-streams NODE_COUNT=10        # Run FloodSub with QUIC streams"
	@echo "  make gossipsub NODE_COUNT=10               # Run GossipSub with 10 nodes"
	@echo "  make rlnc NODE_COUNT=15                    # Run RLNC with 15 nodes"
	@echo "  make rlnc-streams NODE_COUNT=15            # Run RLNC with QUIC streams"
	@echo "  make rs NODE_COUNT=10                      # Run Reed-Solomon with 10 nodes"
	@echo "  make rs-streams NODE_COUNT=10              # Run Reed-Solomon with QUIC streams"
	@echo "  make rlnc MSG_SIZE=256 NUM_CHUNKS=32       # Run RLNC with 32 chunks"
	@echo "  make rs MSG_SIZE=64 NUM_CHUNKS=8           # Run Reed-Solomon with 8 chunks"
	@echo "  make floodsub PROGRESS=true                # Run FloodSub with progress bar"
	@echo "  make test-all                              # Test all existing results"
	@echo ""
	@echo "Individual protocol help:"
	@echo "  cd floodsub && make help                   # FloodSub-specific help"
	@echo "  cd gossipsub && make help                  # GossipSub-specific help"
	@echo "  cd rlnc && make help                       # RLNC-specific help"
	@echo "  cd rs && make help                         # Reed-Solomon-specific help"
