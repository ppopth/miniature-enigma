# Network Topologies for Shadow Simulations

The Shadow simulations now support loading network topologies from JSON files, providing flexible network structures for testing broadcast protocols.

## Architecture

The topology system consists of:

1. **Simple Topology Structure**: A single `Topology` struct containing node count and connection list
2. **Topology Generator**: A separate tool (`topology/gen/topology-gen`) that creates topology files
3. **File-Based Configuration**: Simulations read topology from JSON files

## Topology File Format

Topology files are simple JSON with:
- `node_count`: Total number of nodes
- `connections`: Array of `{from, to}` pairs representing bidirectional connections

Example:
```json
{
  "node_count": 5,
  "connections": [
    {"from": 0, "to": 1},
    {"from": 1, "to": 2},
    {"from": 2, "to": 3},
    {"from": 3, "to": 4}
  ]
}
```

## Using the Topology Generator

### Basic Usage
```bash
# Build the generator
cd topology/gen && go build -o topology-gen main.go

# Generate different topology types with standard naming
./topology-gen -type ring -nodes 10 -output ../topology-10.json
./topology-gen -type mesh -nodes 5 -output ../topology-5.json
```

### Available Topology Types

1. **linear**: Linear chain (0-1-2-3-...)
2. **ring**: Ring with wraparound (0-1-2-...-N-1-0)
3. **mesh**: Fully connected (every node connected to every other)
4. **tree**: Hierarchical tree structure
5. **small-world**: Watts-Strogatz small-world model
6. **random-regular**: Random regular graph with uniform degree distribution

### Parameters

- **All types**: `-nodes N` (number of nodes), `-output filename.json`
- **tree**: `-branching N` (children per node)
- **small-world**: `-k N` (nearest neighbors), `-rewire P` (rewiring probability)
- **random-regular**: `-degree N` (connections per node)

### Visualization and Statistics

```bash
# Generate with visualization (for small networks)
./topology/gen/topology-gen -type ring -nodes 8 -visualize -output ring-8.json

# Generate with statistics
./topology/gen/topology-gen -type tree -nodes 15 -branching 3 -stats -output tree-15.json
```

## Running Simulations with Topology Files

Simulations automatically use topology files with standard naming:

```bash
# Simulations automatically look for topology/topology-{NODE_COUNT}.json
make floodsub NODE_COUNT=10    # Uses topology/topology-10.json if it exists
make rlnc NODE_COUNT=15        # Uses topology/topology-15.json if it exists
make rs NODE_COUNT=5           # Uses topology/topology-5.json if it exists
```

**Automatic Detection:**
- ✅ **Found**: `"Using topology file: /absolute/path/to/topology-10.json"`
- ⚠️ **Not found**: `"No topology file found at ../topology/topology-10.json, using default topology"`

If no topology file is found, simulations use the default topology (linear chain).

## Example Topology Files

Generated topology files are stored in `shadow/topology/` with standard naming:

- `topology-10.json`: 10-node topology (ring/mesh/tree/etc.)
- `topology-15.json`: 15-node topology
- `topology-5.json`: 5-node topology

**Complete Workflow:**
```bash
# 1. Generate topology
cd topology/gen && ./topology-gen -type ring -nodes 10 -output ../topology-10.json

# 2. Run simulation - automatically uses the topology file
cd ../.. && make floodsub NODE_COUNT=10
```

## Creating Custom Topologies

You can manually create topology JSON files or modify generated ones:

```json
{
  "node_count": 4,
  "connections": [
    {"from": 0, "to": 1},
    {"from": 0, "to": 2},
    {"from": 1, "to": 3},
    {"from": 2, "to": 3}
  ]
}
```

## Topology Characteristics

| Type | Connectivity | Fault Tolerance | Path Length | Use Case |
|------|-------------|-----------------|-------------|----------|
| Linear | Low | Poor | High | Simple broadcast chain |
| Ring | Medium | Fair | Medium | Resilient to single failures |
| Mesh | High | Excellent | Low | Maximum resilience |
| Tree | Hierarchical | Poor | Medium | Structured networks |
| Small-World | Medium-High | Good | Low | Real-world networks |
| Random-Regular | Uniform | Good | Medium | Balanced load distribution |

## Benefits of File-Based Approach

1. **Automatic Detection**: No need to specify topology file paths manually
2. **Separation of Concerns**: Topology generation separate from simulation logic
3. **Easy Extension**: Add new topology types without changing simulation code
4. **Reproducibility**: Save and reuse exact network configurations with standard naming
5. **Custom Networks**: Hand-craft specific topologies for testing
6. **Version Control**: Track topology changes alongside code changes
7. **Clear Feedback**: System tells you exactly which topology file is being used

**Standard Naming Convention:**
- Topology files: `topology/topology-{NODE_COUNT}.json`
- Automatic discovery based on `NODE_COUNT` parameter
- Graceful fallback to default topology if file not found

This design makes it easy to experiment with different network structures while providing a consistent, user-friendly interface.
