# Shadow Network Simulations

This directory contains Shadow network simulations for testing eth-ec-broadcast protocols under different network topologies.

## Protocols

- **FloodSub**: Basic message flooding protocol (baseline)
- **GossipSub**: libp2p GossipSub protocol (go-libp2p-pubsub)
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
make floodsub NODE_COUNT=10 MSG_SIZE=256 MSG_COUNT=5
make floodsub-streams NODE_COUNT=10 MSG_SIZE=256 MSG_COUNT=5  # FloodSub with QUIC streams
make gossipsub NODE_COUNT=10 MSG_SIZE=256 MSG_COUNT=5          # libp2p GossipSub
make rlnc NODE_COUNT=10 MSG_SIZE=256 MSG_COUNT=5
make rs NODE_COUNT=10 MSG_SIZE=256 MSG_COUNT=5

# Or run all protocols with same node count
make all NODE_COUNT=10 MSG_SIZE=256 MSG_COUNT=5

# Test results
make test-all NODE_COUNT=10 MSG_COUNT=5

# Run comprehensive topology testing (all topologies × all protocols with retry logic)
make all-topologies LOG_LEVEL=debug
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
├── compare_protocols.py   # Protocol performance comparison tool
├── topology/              # Application topology system
│   ├── topology.go        # Core topology data structure
│   ├── generators.go      # Topology generation functions
│   └── gen/               # Topology generator tool
├── floodsub/              # FloodSub simulation + Makefile
├── gossipsub/             # GossipSub (libp2p) simulation + Makefile
├── rlnc/                  # RLNC simulation + Makefile
├── rs/                    # Reed-Solomon simulation + Makefile
├── TOPOLOGY.md            # Detailed topology documentation
└── COMPARISON.md          # Protocol comparison guide
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
| `MSG_SIZE` | 256 | Message size in bytes |
| `MSG_COUNT` | 5 | Number of messages to publish |
| `NUM_CHUNKS` | 8 | Number of chunks for RLNC and Reed-Solomon |
| `LOG_LEVEL` | info | Log level: debug, info, warn, error |
| `PROGRESS` | false | Show Shadow progress bar during simulation |

### Log Levels

All Shadow simulations support configurable log levels with microsecond-precision timestamps:

- **debug**: Detailed debugging information (chunk validation, peer messages, state changes)
- **info**: General informational messages (default)
- **warn**: Warning messages
- **error**: Error messages only

The log level controls both application logging and go-log subsystems (including libp2p for GossipSub).

**Examples:**
```bash
# Run with debug logging for detailed output
make floodsub LOG_LEVEL=debug NODE_COUNT=10

# Run all protocols with minimal logging
make all LOG_LEVEL=error

# Run comprehensive tests with debug logging
make all-topologies LOG_LEVEL=debug
```

## Transport Modes

### Custom Protocols (FloodSub, RLNC, Reed-Solomon)
These protocols support two QUIC transport modes:
- **Datagrams** (default): Unreliable, unordered delivery with lower latency
- **Streams**: Reliable, ordered delivery with flow control

FloodSub includes both transport modes in CI testing. RLNC and Reed-Solomon currently use datagrams by default but can be configured to use streams via the `--use-streams` flag.

### libp2p Protocol (GossipSub)
GossipSub is a separate implementation using go-libp2p-pubsub with standard TCP transport:
- Uses libp2p's TCP with multiplexed streams (not the custom QUIC host)
- Reliable, ordered delivery with built-in flow control
- Production-tested libp2p transport layer
- Run with: `make gossipsub`

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

## Protocol Performance Comparison

To compare message arrival time performance across GossipSub, Reed-Solomon, and RLNC protocols, see [COMPARISON.md](COMPARISON.md) for the `compare_protocols.py` script.

## Comprehensive Testing

### all-topologies Target

The `all-topologies` target runs exhaustive testing across all topology types and protocols with automatic retry logic for flaky tests:

```bash
# Run all topology/protocol combinations with retry logic
cd shadow && make all-topologies LOG_LEVEL=debug
```

**What it tests:**
- 6 topology types: linear, ring, mesh, tree, small-world, random-regular
- 7 protocol variants: FloodSub (datagrams/streams), GossipSub, RLNC (datagrams/streams), RS (datagrams/streams)
- 42 total test combinations

**Retry Logic:**
- Each test automatically retries up to 5 times on failure
- Cleans simulation data between retries
- 2-second delay between retry attempts
- Fails only if all 5 attempts fail

**Example Output:**
```
[Attempt 1/5] Linear + FloodSub (datagrams)
✓ Linear + FloodSub (datagrams) passed
[Attempt 1/5] Linear + FloodSub (streams)
✗ Test failed, retrying...
[Attempt 2/5] Linear + FloodSub (streams)
✓ Linear + FloodSub (streams) passed
```

This target is used in CI to ensure reliability across all protocol and topology combinations.
