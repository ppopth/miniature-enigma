# Benchmark Pedersen (Rust)

A high-performance Rust implementation of Pedersen commitment benchmarking using `curve25519-dalek` and `blst`.

## Overview

This tool benchmarks the performance of Pedersen commitment operations used in the erasure coding verification system. It measures:

1. **GenerateExtras**: Time to generate Pedersen commitments for all network chunks and sign them with BLS
2. **Verify**: Time to verify a chunk using Pedersen commitments and BLS signature

## Building

```bash
# Build in release mode (optimized)
cargo build --release

# The binary will be at: target/release/benchmark-pedersen
```

## Usage

```bash
# Run with default parameters
./target/release/benchmark-pedersen

# Custom parameters
./target/release/benchmark-pedersen \
  --chunk-size 1024 \
  --num-chunks 8 \
  --iterations 100 \
  --output pedersen_benchmark_rust.json
```

### Command-line Options

- `--chunk-size <bytes>`: Message chunk size in bytes (must be multiple of 32). Default: 1024
- `--num-chunks <count>`: Number of chunks to benchmark. Default: 8
- `--iterations <count>`: Number of iterations per benchmark. Default: 100
- `--output <file>`: Output file for benchmark results. Default: `pedersen_benchmark_rust.json`

## Output Format

The tool outputs benchmark results in JSON format:

```json
{
  "chunk_size": 1056,
  "elements_per_chunk": 32,
  "num_chunks": 8,
  "iterations": 100,
  "generate_extras_ns": 8343322,
  "verify_ns": 2374984
}
```

Fields:
- `chunk_size`: Network chunk size in bytes (includes field element encoding overhead)
- `elements_per_chunk`: Number of field elements per chunk
- `num_chunks`: Number of chunks benchmarked
- `iterations`: Number of iterations performed
- `generate_extras_ns`: Average time for GenerateExtras operation in nanoseconds
- `verify_ns`: Average time for Verify operation in nanoseconds

## Implementation Details

### Pedersen Commitments

The implementation uses Ristretto255 group for Pedersen commitments:

```
C = Σ(vi * Gi)
```

Where:
- `vi` are field elements (scalars) from chunk data
- `Gi` are deterministic generator points
- `C` is the commitment point

### BLS Signatures

Commitments are signed using BLS12-381 signatures:
1. All commitments are hashed together using SHA-256
2. The hash is signed with a BLS secret key
3. Verification checks the signature against the public key

### Verification Process

When verifying a chunk:
1. Parse extra data to extract commitments and BLS signature
2. Verify BLS signature over all commitments
3. Compute Pedersen commitment for the chunk data
4. Compute linear combination of stored commitments using coefficients
5. Verify that computed commitment matches expected commitment

## Performance Comparison

Benchmark with default parameters (1024 byte chunks, 8 chunks, 100 iterations):

| Operation       | Rust (curve25519-dalek) | Go (Ristretto255) | Speedup |
|----------------|-------------------------|-------------------|---------|
| GenerateExtras | 8.34 ms                 | 14.91 ms          | 1.79x   |
| Verify         | 2.37 ms                 | 3.40 ms           | 1.43x   |

The Rust implementation benefits from:
- Highly optimized curve25519-dalek library with SIMD instructions
- Zero-cost abstractions and efficient memory management
- Platform-specific optimizations (AVX2, etc.)
