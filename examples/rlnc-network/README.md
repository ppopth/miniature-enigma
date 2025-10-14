# RLNC Network Example

This example demonstrates a Random Linear Network Coding (RLNC) enabled peer-to-peer network. RLNC provides robust erasure coding that can handle dynamic network topologies and partial message loss.

## Features

- **RLNC Erasure Coding**: Messages are encoded using random linear combinations over finite fields
- **Prime Field**: Uses GF(4294967311) for optimal performance
- **Interactive Interface**: Send messages interactively and see real-time statistics
- **Multi-peer Support**: Connect to multiple peers simultaneously
- **Chunk Reconstruction**: Automatic message reconstruction from received coded chunks

## Build

From the `eth-ec-broadcast/examples/rlnc-network` directory:

```bash
go build
```

## Usage

### Start a listening node
```bash
./rlnc-network -l 8001 -id alice
```

### Connect a second node
```bash
./rlnc-network -l 8002 -c 127.0.0.1:8001 -id bob
```

### Connect a third node to both
```bash
./rlnc-network -l 8003 -c 127.0.0.1:8001,127.0.0.1:8002 -id charlie
```

## Configuration

The RLNC parameters are optimized for small message processing:

- **Field**: Prime field GF(4294967311) - a 32-bit prime for efficient arithmetic
- **Message Chunk Size**: 8 bytes - optimized for short messages and low latency
- **Network Chunk Size**: 9 bytes - minimal overhead for efficient transmission
- **Elements per Chunk**: 2 field elements - compact encoding
- **Coefficient Bits**: 32 bits - full field size for maximum security
- **Publish Multiplier**: 4x - aggressive initial redundancy for reliability
- **Forward Multiplier**: 8x - high forwarding redundancy for network resilience
- **Completion Signals**: Enabled by default - nodes broadcast completion when reconstruction finishes, reducing unused chunk overhead

## Command Line Options

- `-l <port>` - Listening port (required)
- `-c <addresses>` - Comma-separated list of remote addresses to connect to
- `-id <name>` - Node identifier for logging (default: node-<port>)
- `-topic <name>` - Topic name to join (default: rlnc-demo)
- `-v` - Verbose mode for detailed logging

## Interactive Commands

Once running, you can use these commands:

- `/help` - Show available commands
- `/stats` - Display encoding statistics
- `/messages` - List all received message IDs
- `/chunks` - Show chunk counts and reconstruction status
- `quit` or `exit` - Shutdown the node

## Example Session

**Terminal 1 (Alice):**
```
./rlnc-network -l 8001 -id alice
[alice] Host started, listening on 127.0.0.1:8001
[alice] Using prime field GF(4294967311) for optimal performance
[alice] Joined topic 'rlnc-demo' with RLNC erasure coding
[alice] RLNC Config: ChunkSize=64

=== RLNC Network Demo ===
Field: Prime GF(4294967311)
Topic: rlnc-demo
Node: alice

Type messages to broadcast (press Enter to send, 'quit' to exit):
> Hello from Alice
[alice] Publishing message: Hello from Alice
[alice] ✓ Message published successfully (took 1.2ms)
[alice] ✓ Received message: Hello from Bobby
```

**Terminal 2 (Bob):**
```
./rlnc-network -l 8002 -c 127.0.0.1:8001 -id bob
[bob] Host started, listening on 127.0.0.1:8002
[bob] Using prime field GF(4294967311) for optimal performance
[bob] Connecting to 127.0.0.1:8001...
[bob] Successfully connected to 127.0.0.1:8001
[bob] Joined topic 'rlnc-demo' with RLNC erasure coding
[bob] ✓ Received message: Hello from Alice
> Hello from Bobby
[bob] Publishing message: Hello from Bobby
[bob] ✓ Message published successfully (took 0.8ms)
```

## How RLNC Works

1. **Message Chunking**: Messages are split into 8-byte chunks for low-latency processing
2. **Linear Coding**: Each chunk is encoded as a random linear combination of original chunks
3. **Network Distribution**: Coded chunks are distributed with 4x initial redundancy and 8x forwarding redundancy
4. **Reconstruction**: Receivers collect linearly independent chunks to reconstruct the message
5. **Progressive Decoding**: Partial reconstruction is possible as chunks arrive
6. **Completion Signaling**: When a node completes reconstruction, it notifies peers to stop sending chunks for that message, reducing network overhead

## Network Topology Testing

### Star Topology
```bash
# Central hub
./rlnc-network -l 8000 -id hub

# Leaf nodes connecting to hub
./rlnc-network -l 8001 -c 127.0.0.1:8000 -id leaf1
./rlnc-network -l 8002 -c 127.0.0.1:8000 -id leaf2
./rlnc-network -l 8003 -c 127.0.0.1:8000 -id leaf3
```

### Mesh Topology
```bash
# Node 1
./rlnc-network -l 8001 -id node1

# Node 2 connects to Node 1
./rlnc-network -l 8002 -c 127.0.0.1:8001 -id node2

# Node 3 connects to both Node 1 and Node 2
./rlnc-network -l 8003 -c 127.0.0.1:8001,127.0.0.1:8002 -id node3

# Node 4 connects to all previous nodes
./rlnc-network -l 8004 -c 127.0.0.1:8001,127.0.0.1:8002,127.0.0.1:8003 -id node4
```

## Performance Notes

- **Prime Field GF(4294967311)**: Optimized for 32-bit arithmetic operations
- **8-byte Chunks**: Small chunks for efficient processing of short messages
- **4x Publish Redundancy**: Provides robust initial message distribution
- **8x Forward Redundancy**: Aggressive forwarding for network resilience
- **Random Coefficients**: Ensures high probability of linear independence

## Troubleshooting

**Connection Issues:**
- Ensure ports are not blocked by firewall
- Check that addresses are reachable
- Use verbose mode (`-v`) for detailed logging

**Reconstruction Failures:**
- Verify all nodes are using the same topic
- Check network connectivity between peers
- Monitor chunk statistics with `/chunks` command

**Performance Issues:**
- The prime field is optimized for general use
- For specific applications, custom parameters may be needed
- Monitor encoding statistics with `/stats` command
