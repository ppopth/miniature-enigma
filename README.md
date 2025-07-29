# Ethereum Erasure Coding Broadcast

A Go-based peer-to-peer (P2P) broadcast system implementing erasure coding protocols for efficient and resilient message distribution in Ethereum networks.

## Features

- **FloodSub**: Simple flooding protocol for baseline message propagation
- **RLNC (Random Linear Network Coding)**: Advanced erasure coding protocol using linear combinations over finite fields for efficient and resilient broadcast
- **Finite Field Arithmetic**: Generic field operations supporting prime fields and binary fields for erasure coding
- **Chunk Verification**: Pluggable verification system for erasure-coded chunks
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

### Using the Example

```bash
cd examples/simple

# Start a listening node on port 8001
./simple -l 8001

# In another terminal, connect to the listening node
./simple -c 127.0.0.1:8001
```

## Development

### Make Commands

```bash
make help           # Show all available commands
make deps           # Download dependencies
make build          # Build all packages
make test           # Run tests
make coverage       # Run tests with coverage
make lint           # Run linter
make proto          # Generate protobuf files
make example        # Build example application
make ci             # Run full CI pipeline locally
```

### CI/CD

This project uses GitHub Actions for continuous integration:

- **Testing**: Runs tests on Go 1.21, 1.22, and 1.23
- **Linting**: Uses golangci-lint for code quality
- **Building**: Ensures all packages and examples build correctly
- **Protocol Buffers**: Validates generated protobuf files are up to date
- **Security**: Runs Gosec security scanner

## Architecture

### Core Components

- **host/**: Low-level QUIC networking with TLS authentication
- **pubsub/**: Publish-subscribe system managing topics and peer communication
- **floodsub/**: FloodSub router implementing message flooding with deduplication (baseline protocol)
- **rlnc/**: Random Linear Network Coding router implementing erasure coding over finite fields
- **rlnc/field/**: Finite field arithmetic library supporting prime and binary fields for erasure coding
- **rlnc/verify/**: Pluggable chunk verification system for erasure-coded data
- **pb/**: Protocol buffer definitions for RPC communication

### Message Flow

1. **Host Layer**: Establishes QUIC connections between peers
2. **PubSub Layer**: Manages topic subscriptions and routes messages
3. **Router Layer**: Implements specific broadcast algorithms (FloodSub or RLNC)
4. **Application**: Publishes/subscribes to topics through the router

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.
