# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go-based peer-to-peer (P2P) broadcast system implementing multiple routing protocols for message distribution. The project provides a networking stack with two primary routing algorithms:

- **FloodSub**: A simple flooding protocol for message propagation
- **RLNC (Random Linear Network Coding)**: An advanced protocol that uses linear combinations of message chunks for efficient and resilient broadcast

## Architecture

### Core Components

- **host/**: Low-level network host implementation using QUIC transport with TLS certificates for peer authentication
- **pubsub/**: Publish-subscribe system managing topics, subscriptions, and peer communication via RPC messages  
- **floodsub/**: FloodSub router implementing basic message flooding with time-based deduplication
- **rlnc/**: RLNC router performing message chunking, linear combinations over finite fields, and reconstruction
- **rlnc/field/**: Generic finite field interface and implementations (prime fields, matrix operations)
- **pb/**: Protocol buffer definitions for RPC communication between peers

### Protocol Flow

1. **Host Layer**: Establishes QUIC connections between peers using self-signed TLS certificates
2. **PubSub Layer**: Manages topic subscriptions and routes messages to appropriate routers
3. **Router Layer**: Implements specific broadcast algorithms (FloodSub or RLNC)
4. **Message Flow**: Applications publish messages → router processes → distributed to subscribed peers

## Development Commands

### Building
```bash
go build                    # Build current package
go build ./examples/simple  # Build example application
```

### Testing
```bash
go test ./...              # Run all tests
go test -v ./host          # Run host tests with verbose output
go test -v ./pubsub        # Run pubsub tests with verbose output
go test -v ./floodsub      # Run floodsub tests with verbose output
go test -v ./rlnc          # Run RLNC tests with verbose output
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

## Key Implementation Details

### RLNC Algorithm
The RLNC implementation uses finite field arithmetic for message chunking and linear combinations. Key parameters:
- **MessageChunkSize**: 1024 bytes per chunk
- **Field**: Generic finite field interface (default: prime field with 2^256+297)
- **MaxCoefficientBits**: Controls coefficient size for linear combinations
- **Multipliers**: Control redundancy levels for publishing and forwarding

### Verification System
The codebase includes a pluggable chunk verification system in `rlnc/verify/` that allows applications to insert custom validation logic for message chunks.

### Connection Management
The host layer handles peer connections through QUIC with automatic peer discovery via TLS certificate validation and peer ID derivation.

## Module Structure

The project is organized as `github.com/ppopth/p2p-broadcast` with clear separation between networking primitives (host), messaging coordination (pubsub), and routing algorithms (floodsub, rlnc).