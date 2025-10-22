# Examples

This directory contains practical examples demonstrating the erasure coding capabilities of the eth-ec-broadcast library.

## Available Examples

### 1. RLNC Network (`rlnc-network/`)
Random Linear Network Coding implementation for dynamic, resilient networks.

**Features:**
- Random linear combinations over prime field GF(4294967311)
- Interactive message broadcasting
- Real-time coding statistics
- Handles dynamic network topologies

**Usage:**
```bash
cd rlnc-network/
go build
./rlnc-network -l 8001 -id alice
./rlnc-network -l 8002 -c 127.0.0.1:8001 -id bob
```

### 2. Reed-Solomon Network (`rs-network/`)
Reed-Solomon systematic MDS codes for efficient, predictable erasure coding.

**Features:**
- Systematic encoding (original data preserved)
- Binary field GF(2^8) for efficient byte operations
- MDS property: any k chunks can reconstruct k original chunks
- Configurable redundancy ratio
- Interactive message broadcasting with reconstruction statistics

**Usage:**
```bash
cd rs-network/
go build
./rs-network -l 8001 -id alice
./rs-network -l 8002 -c 127.0.0.1:8001 -id bob
```

## Quick Start Guide

### Build All Examples
```bash
# From the repository root
make example        # Build all example applications
make example-linux  # Cross compile for Linux

# Or build individually
cd examples/rlnc-network && go build
cd examples/rs-network && go build
```

### Test Network Topologies

#### Star Topology
```bash
# Central hub
./rlnc-network -l 8000 -id hub

# Leaf nodes
./rlnc-network -l 8001 -c 127.0.0.1:8000 -id leaf1
./rlnc-network -l 8002 -c 127.0.0.1:8000 -id leaf2
./rlnc-network -l 8003 -c 127.0.0.1:8000 -id leaf3
```

#### Mesh Topology
```bash
# Build mesh by connecting each new node to all previous nodes
./rs-network -l 8001 -id node1
./rs-network -l 8002 -c 127.0.0.1:8001 -id node2
./rs-network -l 8003 -c 127.0.0.1:8001,127.0.0.1:8002 -id node3
./rs-network -l 8004 -c 127.0.0.1:8001,127.0.0.1:8002,127.0.0.1:8003 -id node4
```

## Testing

Test network functionality:

```bash
# Test individual examples
cd examples/rlnc-network && ./test.sh
cd examples/rs-network && ./test.sh

# Test all examples from repository root
make test-examples
```
