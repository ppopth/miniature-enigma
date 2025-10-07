# Protocol Performance Comparison

This document describes how to use the `compare_protocols.py` script to compare message arrival time performance across GossipSub (baseline), Reed-Solomon, and RLNC protocols.

## Overview

The comparison script runs Shadow simulations for all three protocols and generates a CDF (Cumulative Distribution Function) plot of message arrival times to compare their performance characteristics.

- **GossipSub**: Baseline protocol using libp2p TCP (shown as dashed line)
- **Reed-Solomon**: Erasure coding improvement using QUIC streams (shown as solid line)
- **RLNC**: Erasure coding improvement using QUIC streams (shown as solid line)

**Transport Modes:**
- GossipSub: libp2p TCP (reliable, ordered) - baseline
- Reed-Solomon: QUIC streams (reliable, ordered)
- RLNC: QUIC streams (reliable, ordered)

## Prerequisites

```bash
pip3 install matplotlib numpy networkx pyyaml
```

## Usage

### Basic Usage

```bash
cd shadow
python3 compare_protocols.py --msg-size 256 --num-chunks 8
```

### Advanced Options

```bash
# Compare protocols with custom parameters
python3 compare_protocols.py --msg-size 512 --num-chunks 16 --node-count 20

# Run with debug logging for detailed output
python3 compare_protocols.py --msg-size 1024 --num-chunks 32 --log-level debug

# Specify custom output file
python3 compare_protocols.py --msg-size 256 --num-chunks 8 -o results/comparison.png
```

### Command-Line Options

| Option | Required | Default | Description |
|--------|----------|---------|-------------|
| `--msg-size` | Yes | - | Message size in bytes |
| `--num-chunks` | Yes | - | Number of chunks for RS and RLNC |
| `--node-count` | No | 10 | Number of nodes in simulation |
| `--msg-count` | No | 1 | Number of messages to send |
| `--degree` | No | 3 | Node degree for random regular topology |
| `--log-level` | No | info | Log level: debug, info, warn, error |
| `-o, --output` | No | protocol_comparison_cdf.png | Output file for CDF plot |

## How It Works

1. **Generate Topology**: Creates a random regular graph topology for all simulations
2. **Run Simulations**: Executes Shadow simulations for GossipSub, Reed-Solomon, and RLNC sequentially
3. **Parse Logs**: Extracts message arrival timestamps from Shadow logs
4. **Calculate Latencies**: Computes latency = arrival_time - first_arrival_time for each message
5. **Generate CDF**: Creates cumulative distribution function from latency data
6. **Plot Results**: Generates comparison plot with GossipSub as baseline
7. **Print Statistics**: Displays mean, median, P95, P99 for each protocol

**Network Topology:**
All simulations use a random regular graph topology where each node has the same degree (number of connections). The default degree is 3, but can be customized with the `--degree` parameter.

## Output

### CDF Plot

The script generates a CDF plot comparing message arrival latencies:

- **X-axis**: Message arrival latency (milliseconds)
- **Y-axis**: Cumulative probability (0-1)
- **GossipSub**: Dashed gray line (baseline)
- **Reed-Solomon**: Solid blue line with circle markers
- **RLNC**: Solid purple line with square markers

### Statistical Summary

The script prints detailed statistics for each protocol:

```
Statistics Summary
============================================================

GossipSub (baseline):
  Count:      100
  Mean:       45.23 ms
  Median:     42.10 ms
  P95:        78.50 ms
  P99:        95.30 ms
  Min:        5.20 ms
  Max:        102.40 ms

Reed-Solomon:
  Count:      100
  Mean:       38.15 ms
  Median:     35.80 ms
  P95:        65.20 ms
  P99:        82.10 ms
  Min:        4.50 ms
  Max:        88.90 ms

RLNC:
  Count:      100
  Mean:       40.22 ms
  Median:     37.90 ms
  P95:        68.40 ms
  P99:        85.50 ms
  Min:        4.80 ms
  Max:        92.30 ms
============================================================
```

## Example Workflow

```bash
# Navigate to shadow directory
cd shadow

# Compare protocols with 512-byte messages and 16 chunks
python3 compare_protocols.py --msg-size 512 --num-chunks 16

# Output:
# - Runs GossipSub simulation
# - Runs Reed-Solomon simulation
# - Runs RLNC simulation
# - Parses logs and extracts arrival times
# - Generates protocol_comparison_cdf.png
# - Prints statistical summary
```

## Interpreting Results

### CDF Interpretation

- **Lower curves = Better performance**: Protocols with curves shifted left have lower latencies
- **Steeper curves = More consistent**: Steeper slopes indicate more predictable performance
- **Tail behavior**: The right tail (P95-P99) shows worst-case performance

### Comparing to Baseline

- If RS/RLNC curves are **left** of GossipSub: Improvement in latency
- If RS/RLNC curves are **right** of GossipSub: Degradation in latency
- Compare P95/P99 values to assess tail latency improvements

## Troubleshooting

### No Data Found

If the script reports "No message arrival data found":
1. Check that simulations completed successfully
2. Verify logs contain message arrival timestamps
3. Try running with `--log-level debug` to see detailed output

### Import Errors

If you see `ImportError: No module named 'matplotlib'`:
```bash
pip3 install matplotlib numpy
```

### Simulation Failures

If a simulation fails:
1. Check Shadow simulator is installed correctly
2. Verify Python dependencies are installed
3. Run individual protocol simulations manually to debug
4. Check `shadow.data/` directory for error logs
