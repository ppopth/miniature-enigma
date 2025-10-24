# Reed-Solomon Network Example

This example demonstrates a Reed-Solomon (RS) enabled peer-to-peer network. Reed-Solomon provides systematic Maximum Distance Separable (MDS) erasure coding with predictable reconstruction guarantees.

## Features

- **Reed-Solomon Coding**: Systematic MDS codes with 100% redundancy
- **Binary Field**: Uses GF(2^8) for optimal byte-oriented operations
- **Interactive Interface**: Send messages and monitor reconstruction statistics
- **Systematic Property**: Original data chunks are preserved as-is
- **MDS Property**: Any k chunks can reconstruct a message encoded into n chunks

## Build

From the `eth-ec-broadcast/examples/rs-network` directory:

```bash
go build
```

## Usage

### Start a listening node
```bash
./rs-network -l 8001 -id alice
```

### Connect a second node
```bash
./rs-network -l 8002 -c 127.0.0.1:8001 -id bob
```

### Connect a third node to both
```bash
./rs-network -l 8003 -c 127.0.0.1:8001,127.0.0.1:8002 -id charlie
```

## Configuration

The Reed-Solomon parameters are optimized for reliable communication:

- **Field**: Binary field GF(2^8) - standard for Reed-Solomon implementations
- **Primitive Element**: 3 in GF(2^8)
- **Redundancy**: 100% (1.0 parity ratio)
- **Chunk Size**: 8 bytes - efficient for small to medium messages
- **Network Chunk Size**: 8 bytes
- **Completion Signals**: Enabled by default - nodes broadcast completion when reconstruction finishes, reducing unused chunk overhead

## Command Line Options

- `-l <port>` - Listening port (required)
- `-c <addresses>` - Comma-separated list of remote addresses to connect to
- `-id <name>` - Node identifier for logging (default: node-<port>)
- `-topic <name>` - Topic name to join (default: rs-demo)
- `-v` - Verbose mode for detailed logging

## Interactive Commands

Once running, you can use these commands:

- `/help` - Show available commands
- `/stats` - Display encoding statistics and reconstruction rates
- `/messages` - List all received message IDs
- `/chunks` - Show chunk counts and reconstruction status for each message
- `quit` or `exit` - Shutdown the node

## Example Session

**Terminal 1 (Alice):**
```
./rs-network -l 8001 -id alice
[alice] Host started, listening on 127.0.0.1:8001
[alice] Using binary field GF(2^8) - standard for Reed-Solomon
[alice] Joined topic 'rs-demo' with Reed-Solomon erasure coding
[alice] RS Config: ChunkSize=8 bytes, Redundancy=100.0%, Field=GF(2^8)

=== Reed-Solomon Network Demo ===
Field: Binary GF(2^8)
Topic: rs-demo
Node: alice
Redundancy: 100.0%

Type messages to broadcast (press Enter to send, 'quit' to exit):
> Hello!!!
[alice] Publishing message: Hello!!!
[alice] ✓ Message published successfully (took 1.1ms)
[alice] ✓ Received message: Response
```

**Terminal 2 (Bob):**
```
./rs-network -l 8002 -c 127.0.0.1:8001 -id bob
[bob] Host started, listening on 127.0.0.1:8002
[bob] Connecting to 127.0.0.1:8001...
[bob] Successfully connected to 127.0.0.1:8001
[bob] Using binary field GF(2^8) - standard for Reed-Solomon
[bob] Joined topic 'rs-demo' with Reed-Solomon erasure coding
[bob] ✓ Received message: Hello!!!
> Response
[bob] Publishing message: Response
[bob] ✓ Message published successfully (took 0.9ms)
```

## How Reed-Solomon Works

1. **Message Division**: Messages are divided into k data symbols (e.g., k=1 for "Hello!!!", k=2 for "test-message-123")
2. **Systematic Encoding**: Original data symbols are kept unchanged
3. **Parity Generation**: k parity symbols are generated (100% redundancy)
4. **MDS Property**: Any k symbols (data or parity) can reconstruct the message
5. **Network Distribution**: All 2k symbols are distributed across the network
6. **Reconstruction**: Receivers need any k symbols to perfectly reconstruct
7. **Completion Signaling**: When a node completes reconstruction, it notifies peers to stop sending chunks for that message, reducing network overhead

## Redundancy Configuration

With 100% redundancy (hardcoded):
- k data chunks + k parity chunks = 2k total chunks (k varies by message size)
- Need any k chunks to reconstruct
- Can lose up to k chunks (50% loss tolerance)

## Network Topology Testing

### Star Topology
```bash
# Central hub
./rs-network -l 8000 -id hub

# Leaf nodes
./rs-network -l 8001 -c 127.0.0.1:8000 -id leaf1
./rs-network -l 8002 -c 127.0.0.1:8000 -id leaf2
./rs-network -l 8003 -c 127.0.0.1:8000 -id leaf3
```

### Mesh Topology
```bash
# Build mesh incrementally
./rs-network -l 8001 -id node1
./rs-network -l 8002 -c 127.0.0.1:8001 -id node2
./rs-network -l 8003 -c 127.0.0.1:8001,127.0.0.1:8002 -id node3
./rs-network -l 8004 -c 127.0.0.1:8001,127.0.0.1:8002,127.0.0.1:8003 -id node4
```

## Performance Notes

- **GF(2^8)**: Standard field for Reed-Solomon, optimized for byte operations
- **8-byte chunks**: Small chunks for fine-grained erasure coding
- **100% redundancy**: Provides maximum error correction with 50% loss tolerance
- **Systematic encoding**: Original data can be read directly without decoding

## Use Cases

Reed-Solomon is ideal for:
- Storage systems where systematic property is important
- Networks with predictable topology and loss patterns
- Applications requiring guaranteed reconstruction bounds
- High-throughput scenarios where computational efficiency matters

## Troubleshooting

**Reconstruction Failures:**
- Ensure you have received at least k chunks (where k = message_size / 8)
- Check that all nodes are using the same topic
- Use `/chunks` command to monitor chunk status

**Connection Issues:**
- Check firewall settings
- Ensure ports are available
- Use verbose mode (`-v`) for debugging

**Performance:**
- GF(2^8) is optimized for general Reed-Solomon use
- 32-byte chunks provide good efficiency
- Monitor statistics with `/stats` command
