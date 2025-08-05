# eth-ec-broadcast Shadow Simulation Suite

This directory contains Shadow network simulation setup for testing eth-ec-broadcast protocols under realistic network conditions. The suite supports three broadcast protocols:

- **FloodSub**: Basic message flooding protocol (baseline)
- **RLNC**: Random Linear Network Coding erasure coding
- **Reed-Solomon**: Reed-Solomon erasure coding

## Prerequisites

1. **Shadow Simulator**: Install from https://shadow.github.io/docs/guide/install.html
2. **Python 3** with required packages:
   ```bash
   pip3 install networkx pyyaml
   ```
3. **Go 1.21+** for building the simulation binary

## Quick Start

```bash
# Run all three protocols with default parameters (10 nodes, 5 messages, 64 bytes each)
make all

# Run all protocols with same parameters and test
make all NODE_COUNT=20 MSG_SIZE=128 MSG_COUNT=10
make test-all NODE_COUNT=20 MSG_COUNT=10

# Or run individual protocols with different parameters
make floodsub NODE_COUNT=20 MSG_SIZE=128 MSG_COUNT=10
make rlnc NODE_COUNT=15 MSG_SIZE=64 MSG_COUNT=5
make rs NODE_COUNT=12 MSG_SIZE=32 MSG_COUNT=8

# Clean up all protocol directories
make clean
```

## Files

- `Makefile` - Master coordinator for all protocol simulations
- `network_graph.py` - Generates Shadow network topology and configuration
- `shadow.template.yaml` - Shadow configuration template
- `test_results.py` - Test script for verifying simulation results from Shadow logs
- `floodsub/` - FloodSub protocol simulation (main.go + Makefile)
- `rlnc/` - RLNC erasure coding simulation (main.go + Makefile)
- `rs/` - Reed-Solomon erasure coding simulation (main.go + Makefile)

## Network Topology

The simulation uses a realistic global network topology with:

- **8 geographic regions**: Australia, Europe, East Asia, West Asia, North America (East/West), South Africa, South America
- **2 node types**:
  - `supernode`: 1024 Mbps up/down (20% of nodes)
  - `fullnode`: 50 Mbps up/down (80% of nodes)
- **Realistic latencies** between regions (e.g., 110ms Australia↔East Asia, 70ms Europe↔NA East)

## Simulation Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `NODE_COUNT` | 10 | Number of nodes in simulation |
| `MSG_SIZE` | 64 | Message size in bytes |
| `MSG_COUNT` | 5 | Number of messages to publish |

## How It Works

1. **Network Generation**: `network_graph.py` creates a GML network graph with realistic global topology
2. **Shadow Configuration**: Generates `shadow.yaml` with node placement and process definitions
3. **Simulation**: Each node runs the floodsub protocol, with node 0 publishing messages
4. **Testing**: `test_results.py` parses Shadow logs to verify message delivery

## Example Output

```
Shadow Simulation Test Results
========================================
Node  0:  5/ 5 messages ✓ PASS
Node  1:  5/ 5 messages ✓ PASS
Node  2:  4/ 5 messages ✗ FAIL
----------------------------------------
Total messages received: 14
Expected total: 15
✗ TEST FAILED: 1 nodes failed to receive all messages
Failed nodes: [2]
```

## Advanced Usage

### Running Multiple Scenarios

```bash
# Test different network sizes
for nodes in 5 10 20 50; do
    echo "Testing with $nodes nodes"
    make run-sim NODE_COUNT=$nodes
    make test NODE_COUNT=$nodes
    mv shadow.data shadow.data.$nodes
done
```

### Custom Network Topology

Modify `network_graph.py` to adjust:
- Geographic regions and their weights
- Node types and bandwidth allocation
- Inter-region latencies
- Connection patterns

### Testing Results

```bash
# Test simulation results
python3 test_results.py 10 5

# Examine Shadow logs directly
ls shadow.data/hosts/node*/*.stderr
grep "Received message" shadow.data/hosts/node*/*.stderr
```

## Troubleshooting

**Shadow not found**: Install Shadow simulator from https://shadow.github.io/

**Python import errors**: Install required packages:
```bash
pip3 install networkx pyyaml
```

**Build failures**: Ensure Go 1.21+ is installed and `go mod tidy` has been run in the project root

**Test failures**: Check Shadow logs in `shadow.data/hosts/node*/*.stderr` for connection or message delivery errors
