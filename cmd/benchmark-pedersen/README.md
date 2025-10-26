# Pedersen Verifier Benchmark Tool

This tool benchmarks the PedersenVerifier's cryptographic operations (`GenerateExtras` and `Verify`) and outputs timing data to a JSON file. The benchmark results can be used by the ShadowPedersenVerifier to simulate realistic timing in Shadow network simulations without the computational overhead of actual cryptographic operations.

## Usage

```bash
go run ./cmd/benchmark-pedersen [flags]
```

### Flags

- `-chunk-size` - **Message chunk size** in bytes (default: 1024). Must be a multiple of 32. This is the size of the actual data chunks, not the network serialization size.
- `-num-chunks` - Number of chunks to benchmark (default: 8)
- `-iterations` - Number of iterations per benchmark (default: 100)
- `-output` - Output file for benchmark results (default: `pedersen_benchmark.json`)

### Chunk Size Calculations

The tool automatically calculates related values from the message chunk size:

```
elementsPerChunk = messageChunkSize / 32
networkChunkSize = elementsPerChunk × 33
```

- **Message chunk size**: Size of actual data chunks (32 bytes per field element)
- **Network chunk size**: Size after serialization (33 bytes per field element, with 1 extra byte overhead)
- **Elements per chunk**: Number of field elements per chunk

This matches the calculations in `shadow/rlnc/main.go`, where each field element holds 32 bytes (256 bits) of data but requires 33 bytes for network serialization.

## Benchmark Configuration

The tool benchmarks PedersenVerifier operations for a specified number of chunks, measuring:
1. **GenerateExtras**: Time to generate Pedersen commitments and BLS signature for all chunks
2. **Verify**: Time to verify a single chunk's Pedersen commitment and BLS signature

## Example

Generate benchmark data with default settings (8 chunks):
```bash
go run ./cmd/benchmark-pedersen -iterations 100 -output pedersen_benchmark.json
```

Generate benchmark data for 16 chunks:
```bash
go run ./cmd/benchmark-pedersen -num-chunks 16 -iterations 100 -output pedersen_16chunks.json
```

Generate benchmark data with custom chunk size:
```bash
go run ./cmd/benchmark-pedersen -chunk-size 512 -num-chunks 8 -iterations 200 -output custom_benchmark.json
```

## Output Format

The tool outputs a JSON file with the following structure:

```json
{
  "chunk_size": 1056,
  "elements_per_chunk": 32,
  "num_chunks": 8,
  "iterations": 100,
  "generate_extras_ns": 4841752,
  "verify_ns": 2053865
}
```

Fields:
- `chunk_size`: **Network chunk size** in bytes (elements_per_chunk × 33)
- `elements_per_chunk`: Number of field elements per chunk (calculated from message chunk size)
- `num_chunks`: Number of chunks tested
- `iterations`: Number of iterations used
- `generate_extras_ns`: Average time for GenerateExtras in nanoseconds
- `verify_ns`: Average time for Verify in nanoseconds

## Using Benchmark Results

The generated benchmark file can be used with the ShadowPedersenVerifier in Shadow simulations:

### Option 1: Using compare_protocols.py
```bash
cd shadow
python3 compare_protocols.py --benchmark-file ../pedersen_benchmark.json
```

### Option 2: Using the Makefile
```bash
cd shadow
make rlnc-streams BENCHMARK_FILE=../pedersen_benchmark.json
```

### Option 3: Direct RLNC simulation
```bash
cd shadow/rlnc
make all-streams BENCHMARK_FILE=../../pedersen_benchmark.json
```

When a benchmark file is provided, the RLNC simulation uses ShadowPedersenVerifier instead of the real cryptographic verifier, simulating realistic timing using `time.Sleep()` without the computational cost.

## Validation

The tool validates that:
- Chunk size is a multiple of 32 bytes (the size of a field element)
- All cryptographic operations complete successfully

If validation fails, the tool exits with an error message.
