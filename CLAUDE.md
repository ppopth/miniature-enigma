# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go-based peer-to-peer (P2P) broadcast system focused on erasure coding protocols for efficient and resilient message distribution in Ethereum networks. The project provides a networking stack with two primary routing algorithms:

- **FloodSub**: A simple flooding protocol for baseline message propagation
- **EC (Erasure Coding)**: Advanced erasure coding router supporting multiple encoding schemes:
  - **RLNC (Random Linear Network Coding)**: Uses linear combinations over finite fields for efficient and resilient broadcast
  - **Reed-Solomon**: Systematic MDS codes for efficient erasure coding with predictable chunk indices

The networking layer supports both QUIC datagrams (unreliable, unordered) and QUIC streams (reliable, ordered) for flexible transport options. All protocols (FloodSub, RLNC, Reed-Solomon) can use either transport mode.

## Architecture

### Core Components

- **host/**: Low-level network host implementation using QUIC transport with TLS certificates for peer authentication
- **pubsub/**: Publish-subscribe system managing topics, subscriptions, and peer communication via RPC messages  
- **floodsub/**: FloodSub router implementing basic message flooding with time-based deduplication (baseline protocol)
- **ec/**: Erasure coding implementations and utilities
  - **ec/encode/**: Encoder implementations for different erasure coding schemes
    - **ec/encode/rlnc/**: RLNC (Random Linear Network Coding) encoder implementing erasure coding with message chunking, linear combinations over finite fields, and reconstruction
    - **ec/encode/rlnc/verify/**: Pluggable chunk verification system for erasure-coded data (signatures, commitments, etc.)
    - **ec/encode/rs/**: Reed-Solomon encoder implementation with MDS (Maximum Distance Separable) property support
  - **ec/field/**: Generic finite field arithmetic library supporting prime fields, binary fields, and matrix operations for erasure coding
  - **ec/group/**: Group operations for cryptographic primitives (e.g., Ristretto255)
- **pb/**: Protocol buffer definitions for RPC communication between peers

### Protocol Flow

1. **Host Layer**: Establishes QUIC connections between peers using self-signed TLS certificates
2. **PubSub Layer**: Manages topic subscriptions and routes messages to appropriate routers
3. **Router Layer**: Implements specific broadcast algorithms (FloodSub or EC with pluggable encoders)
4. **Encoder Layer**: Handles message chunking, encoding, and reconstruction (RLNC, Reed-Solomon, etc.)
5. **Message Flow**: Applications publish messages → router processes → encoder chunks/encodes → distributed to subscribed peers

## Development Commands

### Make Commands (Recommended)
```bash
make help                  # Show all available commands
make deps                  # Download and verify dependencies
make build                 # Build all packages
make test                  # Run all tests with race detection
make test-short           # Run tests without race detection
make coverage             # Run tests with coverage report
make bench                # Run benchmarks for all packages
make bench-rlnc           # Run RLNC encoder benchmarks
make bench-rs             # Run Reed-Solomon encoder benchmarks
make bench-field          # Run field arithmetic benchmarks
make test-examples        # Test example applications
make proto                # Generate protobuf files
make proto-clean          # Clean protobuf generated files
make example              # Build all example applications
make example-linux        # Cross compile examples for Linux
make install-tools        # Install development tools
make clean                # Clean build artifacts
```

### Direct Go Commands
```bash
go build                    # Build current package
go build ./examples/simple  # Build example application
go test ./...              # Run all tests
go test -v ./host          # Run host tests with verbose output
go test -v ./pubsub        # Run pubsub tests with verbose output
go test -v ./floodsub      # Run floodsub tests with verbose output
go test -v ./ec/encode/rlnc  # Run RLNC tests with verbose output
go test -race -coverprofile=coverage.out ./...  # Tests with coverage
```

### Protocol Buffers
```bash
cd pb && make              # Generate Go code from .proto files
cd pb && make clean        # Clean generated files
```

### Running Examples
```bash
# Simple Network (basic P2P)
cd examples/simple && go build
./simple -l 8001          # Listening node
./simple -c 127.0.0.1:8001 # Connecting node

# RLNC Network (Random Linear Network Coding)
cd examples/rlnc-network && go build
./rlnc-network -l 8001 -id alice
./rlnc-network -l 8002 -c 127.0.0.1:8001 -id bob

# Reed-Solomon Network (Systematic MDS Codes)
cd examples/rs-network && go build
./rs-network -l 8001 -id alice
./rs-network -l 8002 -c 127.0.0.1:8001 -id bob
```


## Recent Code Improvements

### Code Quality Enhancements
The codebase has been significantly improved with:
- **Variable Renaming**: More descriptive variable names throughout all packages
- **Comprehensive Comments**: All functions, types, and complex logic now have clear documentation
- **Consistent Naming**: Following Go conventions and patterns across the entire codebase
- **Better Readability**: Enhanced code structure and organization

### Improved Packages
- **host/**: Renamed variables (h→host, lk→mutex, peers→connections, etc.) and added comprehensive comments
- **pubsub/**: Renamed variables (ps→pubsub, lk→mutex, topics→topicSubscriptions, etc.) and documented all methods
- **floodsub/**: Renamed variables (fs→router, lk→mutex, peers→peerSendFuncs, etc.) and added detailed comments

### Recent Algorithm Improvements
- **Matrix Operations**: Fixed InvertMatrix implementation with proper row pivoting for numerical stability
- **Reed-Solomon Encoder**: Now correctly satisfies the MDS (Maximum Distance Separable) property for optimal error correction
- **Field Support**: Added comprehensive support for both binary fields (GF(2^m)) and prime fields in the Reed-Solomon encoder
- **Erasure Coding**: Enhanced implementation ensures proper systematic encoding and decoding capabilities

## CI/CD Pipeline

### GitHub Actions Workflow
The project includes a CI/CD pipeline with:
- **Multi-version testing**: Go 1.21, 1.22, and 1.23
- **Automated testing**: All tests run on each push and pull request
- **Build validation**: Ensures all packages build correctly
- **Code formatting**: Validates code formatting with `gofmt`
- **Example testing**: Tests both RLNC and Reed-Solomon network examples for network functionality
- **Shadow simulations**: Tests FloodSub, GossipSub, RLNC, and Reed-Solomon protocols under realistic network conditions

### Code Quality Tools
- **Makefile**: Standardized development workflow commands
- **Coverage reporting**: Local coverage analysis and reporting via make commands
- **Protocol buffers**: Automated generation and validation

## Key Implementation Details

### Erasure Coding Encoders

#### RLNC Algorithm
The RLNC implementation uses finite field arithmetic for message chunking and linear combinations. Key parameters:
- **ChunkSize**: 1024 bytes per chunk
- **Field**: Generic finite field interface (default: prime field with 2^256+297)
- **MaxCoefficientBits**: Controls coefficient size for linear combinations
- **Multipliers**: Control redundancy levels for publishing and forwarding

#### Reed-Solomon Algorithm
The Reed-Solomon implementation provides systematic MDS codes with predictable chunk indices:
- **ParityRatio**: Configurable redundancy ratio (e.g., 0.5 = 50% redundancy)
- **MessageChunkSize**: Size of original data chunks
- **ElementsPerChunk**: Number of field elements per chunk
- **Systematic encoding**: First k chunks are original data, remaining are parity chunks

### Verification System
The codebase includes a pluggable chunk verification system in `ec/encode/rlnc/verify/` that allows applications to insert custom validation logic for message chunks. Current implementations include:
- **Pedersen commitments**: Cryptographic verification using Ristretto255 group operations

### Connection Management
The host layer handles peer connections through QUIC with automatic peer discovery via TLS certificate validation and peer ID derivation.

### Transport Modes
The host supports two QUIC transport modes:
- **TransportDatagram**: Uses QUIC datagrams for unreliable, unordered message delivery (lower latency, no flow control)
- **TransportStream**: Uses QUIC streams for reliable, ordered message delivery (higher latency, flow control, guaranteed delivery)

Configure via `host.WithTransportMode(mode)` option when creating a host.

All Shadow simulations support both transport modes:
- **FloodSub**: `make floodsub` (datagrams), `make floodsub-streams` (streams)
- **RLNC**: `make rlnc` (datagrams), `make rlnc-streams` (streams)
- **Reed-Solomon**: `make rs` (datagrams), `make rs-streams` (streams)
- **GossipSub**: Uses libp2p's TCP transport (separate implementation, not configurable)

## Module Structure

The project is organized as `github.com/ethp2p/eth-ec-broadcast` with clear separation between networking primitives (host), messaging coordination (pubsub), routing algorithms (floodsub), and erasure coding implementations (ec).