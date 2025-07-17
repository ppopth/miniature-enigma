# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go-based peer-to-peer (P2P) broadcast system focused on erasure coding protocols for efficient and resilient message distribution in Ethereum networks. The project provides a networking stack with two primary routing algorithms:

- **FloodSub**: A simple flooding protocol for baseline message propagation
- **RLNC (Random Linear Network Coding)**: An advanced erasure coding protocol that uses linear combinations over finite fields for efficient and resilient broadcast

## Architecture

### Core Components

- **host/**: Low-level network host implementation using QUIC transport with TLS certificates for peer authentication
- **pubsub/**: Publish-subscribe system managing topics, subscriptions, and peer communication via RPC messages  
- **floodsub/**: FloodSub router implementing basic message flooding with time-based deduplication (baseline protocol)
- **rlnc/**: RLNC router implementing erasure coding with message chunking, linear combinations over finite fields, and reconstruction
- **rlnc/field/**: Generic finite field arithmetic library supporting prime fields, binary fields, and matrix operations for erasure coding
- **rlnc/verify/**: Pluggable chunk verification system for erasure-coded data (signatures, commitments, etc.)
- **pb/**: Protocol buffer definitions for RPC communication between peers

### Protocol Flow

1. **Host Layer**: Establishes QUIC connections between peers using self-signed TLS certificates
2. **PubSub Layer**: Manages topic subscriptions and routes messages to appropriate routers
3. **Router Layer**: Implements specific broadcast algorithms (FloodSub or RLNC)
4. **Message Flow**: Applications publish messages → router processes → distributed to subscribed peers

## Development Commands

### Make Commands (Recommended)
```bash
make help                  # Show all available commands
make deps                  # Download and verify dependencies
make build                 # Build all packages
make test                  # Run all tests with race detection
make test-short           # Run tests without race detection
make coverage             # Run tests with coverage report
make lint                 # Run golangci-lint
make proto                # Generate protobuf files
make example              # Build example application
make ci                   # Run full CI pipeline locally
```

### Direct Go Commands
```bash
go build                    # Build current package
go build ./examples/simple  # Build example application
go test ./...              # Run all tests
go test -v ./host          # Run host tests with verbose output
go test -v ./pubsub        # Run pubsub tests with verbose output
go test -v ./floodsub      # Run floodsub tests with verbose output
go test -v ./rlnc          # Run RLNC tests with verbose output
go test -race -coverprofile=coverage.out ./...  # Tests with coverage
```

### Protocol Buffers
```bash
cd pb && make              # Generate Go code from .proto files
cd pb && make clean        # Clean generated files
```

### Running Examples
```bash
cd examples/simple
go build
./simple -l 8001          # Start listening node on port 8001
./simple -c 127.0.0.1:8001 # Connect to listening node
```

### Linting and Code Quality
```bash
golangci-lint run          # Run linter (requires golangci-lint installed)
make lint                  # Run linter via Makefile
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

## CI/CD Pipeline

### GitHub Actions Workflow
The project includes a comprehensive CI/CD pipeline with:
- **Multi-version testing**: Go 1.21, 1.22, and 1.23
- **Race condition detection**: All tests run with `-race` flag
- **Code coverage**: Integrated with Codecov for coverage reporting
- **Linting**: Uses golangci-lint with comprehensive rules
- **Security scanning**: Gosec security vulnerability detection
- **Protocol buffer validation**: Ensures generated files are up to date
- **Cross-platform building**: Validates builds across different environments

### Code Quality Tools
- **golangci-lint**: Comprehensive linting with performance, security, and style checks
- **Makefile**: Standardized development workflow commands
- **Coverage reporting**: Automated coverage analysis and reporting

## Key Implementation Details

### RLNC Algorithm
The RLNC implementation uses finite field arithmetic for message chunking and linear combinations. Key parameters:
- **ChunkSize**: 1024 bytes per chunk
- **Field**: Generic finite field interface (default: prime field with 2^256+297)
- **MaxCoefficientBits**: Controls coefficient size for linear combinations
- **Multipliers**: Control redundancy levels for publishing and forwarding

### Verification System
The codebase includes a pluggable chunk verification system in `rlnc/verify/` that allows applications to insert custom validation logic for message chunks.

### Connection Management
The host layer handles peer connections through QUIC with automatic peer discovery via TLS certificate validation and peer ID derivation.

## Module Structure

The project is organized as `github.com/ethp2p/eth-ec-broadcast` with clear separation between networking primitives (host), messaging coordination (pubsub), and routing algorithms (floodsub, rlnc).