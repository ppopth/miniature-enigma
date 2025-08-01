# Ethereum Erasure Coding Broadcast

A Go-based peer-to-peer (P2P) broadcast system implementing erasure coding protocols for efficient and resilient message distribution in Ethereum networks.

## Features

- **FloodSub**: Simple flooding protocol for baseline message propagation
- **Erasure Coding Router**: Advanced erasure coding with pluggable encoder interface supporting:
  - **RLNC (Random Linear Network Coding)**: Linear combinations over finite fields for efficient and resilient broadcast
  - **Reed-Solomon**: Systematic MDS codes for efficient erasure coding with predictable chunk indices
- **Finite Field Arithmetic**: Generic field operations supporting prime fields and binary fields for erasure coding
- **Group Operations**: Cryptographic primitives including Ristretto255 for verification systems
- **Chunk Verification**: Pluggable verification system with Pedersen commitment support
- **QUIC Transport**: Modern, secure transport layer with TLS authentication
- **Modular Design**: Clean separation between networking, pubsub, and erasure coding layers

## Quick Start

### Prerequisites

- Go 1.21 or later
- Protocol Buffers compiler (for development)

### Building

```bash
# Download dependencies
go mod download

# Build all packages
go build ./...

# Build example application
cd examples/simple
go build
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
```

### Examples

The project includes comprehensive examples demonstrating different erasure coding approaches:

#### Simple Network (Basic P2P)
```bash
cd examples/simple
go build
./simple -l 8001               # Listening node
./simple -c 127.0.0.1:8001     # Connecting node
```

#### RLNC Network (Random Linear Network Coding)
```bash
cd examples/rlnc-network
go build
./rlnc-network -l 8001 -id alice
./rlnc-network -l 8002 -c 127.0.0.1:8001 -id bob
```

#### Reed-Solomon Network (Systematic MDS Codes)
```bash
cd examples/rs-network
go build
./rs-network -l 8001 -id alice
./rs-network -l 8002 -c 127.0.0.1:8001 -id bob
```

See `examples/README.md` for detailed usage instructions, performance comparisons, and advanced configurations.

## Development

### Make Commands

```bash
make help           # Show all available commands
make deps           # Download and verify dependencies
make build          # Build all packages
make test           # Run tests with race detection
make test-short     # Run tests without race detection
make coverage       # Run tests with coverage
make bench          # Run benchmarks for all packages
make bench-rlnc     # Run RLNC encoder benchmarks
make bench-rs       # Run Reed-Solomon encoder benchmarks
make bench-field    # Run field arithmetic benchmarks
make test-examples  # Test example applications
make proto          # Generate protobuf files
make proto-clean    # Clean protobuf generated files
make example        # Build all example applications
make example-linux  # Cross compile examples for Linux
make install-tools  # Install development tools
make clean          # Clean build artifacts
```

### CI/CD

This project uses GitHub Actions for continuous integration:

- **Testing**: Runs tests on Go 1.21, 1.22, and 1.23
- **Building**: Ensures all packages build correctly
- **Code Formatting**: Validates code formatting with `gofmt`
- **Example Testing**: Tests both RLNC and Reed-Solomon network examples for network functionality

## Architecture

### Core Components

- **host/**: Low-level QUIC networking with TLS authentication
- **pubsub/**: Publish-subscribe system managing topics and peer communication
- **floodsub/**: FloodSub router implementing message flooding with deduplication (baseline protocol)
- **ec/**: Erasure coding router and implementations
  - **ec/encode/**: Generic encoder interface and implementations for different erasure coding schemes
    - **ec/encode/rlnc/**: Random Linear Network Coding encoder with linear combinations over finite fields
    - **ec/encode/rlnc/verify/**: Pluggable chunk verification system including Pedersen commitments
    - **ec/encode/rs/**: Reed-Solomon encoder with systematic MDS property for predictable erasure coding
  - **ec/field/**: Finite field arithmetic library supporting prime and binary fields
  - **ec/group/**: Group operations for cryptographic primitives (Ristretto255)
- **pb/**: Protocol buffer definitions for RPC communication

### Message Flow

1. **Host Layer**: Establishes QUIC connections between peers
2. **PubSub Layer**: Manages topic subscriptions and routes messages
3. **Router Layer**: Implements specific broadcast algorithms (FloodSub or EC with pluggable encoders)
4. **Encoder Layer**: Handles message chunking, encoding, and reconstruction (RLNC, Reed-Solomon, etc.)
5. **Application**: Publishes/subscribes to topics through the router

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.
