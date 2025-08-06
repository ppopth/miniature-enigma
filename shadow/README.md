# Shadow Network Simulations

This directory contains Shadow network simulations for testing eth-ec-broadcast protocols under different network topologies.

## Protocols

- **FloodSub**: Basic message flooding protocol (baseline)
- **RLNC**: Random Linear Network Coding erasure coding
- **Reed-Solomon**: Reed-Solomon erasure coding

## Prerequisites

- **Shadow Simulator**: Install from https://shadow.github.io/
- **Python 3** with packages: `pip3 install networkx pyyaml`
- **Go 1.21+**

## Quick Start

### Run Simulations
```bash
# Generate topology files (optional - uses default topology if not found)
cd topology/gen && go build -o topology-gen main.go
./topology-gen -type ring -nodes 10 -output ../topology-10.json
cd ../..

# Run simulations - automatically uses topology/topology-{NODE_COUNT}.json if available
make floodsub NODE_COUNT=10 MSG_SIZE=64 MSG_COUNT=5
make rlnc NODE_COUNT=10 MSG_SIZE=64 MSG_COUNT=5
make rs NODE_COUNT=10 MSG_SIZE=64 MSG_COUNT=5

# Or run all protocols with same node count
make all NODE_COUNT=10 MSG_SIZE=64 MSG_COUNT=5

# Test results
make test-all NODE_COUNT=10 MSG_COUNT=5
```

## Architecture: Two-Layer Network Model

### 1. Shadow Network Layer (Physical Infrastructure)
- **Managed by**: `network_graph.py` → generates `graph.gml`
- **Provides**: Realistic internet latencies, bandwidth, packet loss
- **Geographic regions**: Australia, Europe, Asia, Americas, Africa
- **Node types**: Supernodes (1Gbps), Fullnodes (50Mbps)

### 2. Application Topology Layer (Protocol Connections)
- **Managed by**: JSON topology files from `topology/gen/`
- **Provides**: Application-level connection patterns (who talks to whom)
- **Types**: Linear, ring, mesh, tree, small-world

## How It Works

1. **Shadow Infrastructure**: `network_graph.py` creates realistic network with geographic latencies
2. **Application Connections**: Your topology JSON files define which nodes connect at application level
3. **Combined Effect**: Nodes follow application topology but experience realistic network delays
4. **Simulation**: Shadow runs multiple instances, each with different `-node-id` and `-topology-file`

## Directory Structure

```
shadow/
├── Makefile               # Master build coordinator
├── network_graph.py       # Generates Shadow network infrastructure
├── test_results.py        # Validates simulation results
├── topology/              # Application topology system
│   ├── topology.go        # Core topology data structure
│   ├── generators.go      # Topology generation functions
│   └── gen/               # Topology generator tool
├── floodsub/              # FloodSub simulation + Makefile
├── rlnc/                  # RLNC simulation + Makefile
├── rs/                    # Reed-Solomon simulation + Makefile
└── TOPOLOGY.md            # Detailed topology documentation
```

## Available Application Topologies

1. **linear**: Simple chain (0-1-2-3-...)
2. **ring**: Circular with wraparound
3. **mesh**: Fully connected
4. **tree**: Hierarchical (configurable branching)
5. **small-world**: Watts-Strogatz model
6. **random-regular**: Random graph with uniform degree

See [TOPOLOGY.md](TOPOLOGY.md) for detailed topology documentation.

## Simulation Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `NODE_COUNT` | 10 | Number of nodes in simulation |
| `MSG_SIZE` | 64 | Message size in bytes |
| `MSG_COUNT` | 5 | Number of messages to publish |

## Example: Complete Topology Simulation

```bash
# Generate a ring topology for 10 nodes
cd topology/gen && go build -o topology-gen main.go
./topology-gen -type ring -nodes 10 -output ../topology-10.json

# Run simulation - automatically uses the topology file
cd ../..
make floodsub NODE_COUNT=10 MSG_COUNT=5

# Test results
cd floodsub && make test NODE_COUNT=10 MSG_COUNT=5
```

The simulation will run with:
- **Physical layer**: Realistic global internet latencies (Shadow)
- **Application layer**: Your custom topology connection pattern
