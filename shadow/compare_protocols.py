#!/usr/bin/env python3
"""
Compare message arrival times across GossipSub (baseline), Reed-Solomon, and RLNC protocols.

This script runs Shadow simulations for all three protocols and generates a CDF plot
of message arrival times to compare their performance.

Transport modes:
- GossipSub: libp2p TCP (baseline)
- Reed-Solomon: QUIC streams (reliable, ordered)
- RLNC: QUIC streams (reliable, ordered)
"""

import argparse
import subprocess
import os
import re
import sys
from collections import defaultdict
import matplotlib.pyplot as plt
import numpy as np


def generate_topology(node_count, degree=3):
    """Generate random regular topology for the simulations."""
    print(f"\n{'='*60}")
    print(f"Generating random regular topology ({node_count} nodes, degree {degree})...")
    print(f"{'='*60}")

    topology_file = f"topology-{node_count}.json"
    topology_path = f"topology/{topology_file}"

    # Build topology generator if needed
    gen_dir = "topology/gen"
    gen_binary = os.path.join(gen_dir, "topology-gen")

    if not os.path.exists(gen_binary):
        print("Building topology generator...")
        try:
            subprocess.run(["go", "build", "-o", "topology-gen", "main.go"],
                          cwd=gen_dir, check=True, capture_output=True)
        except subprocess.CalledProcessError as e:
            print(f"✗ Build failed:")
            print(e.stderr.decode())
            return False

    # Generate random regular topology
    cmd = [f"./{gen_binary}",
           "-type", "random-regular",
           "-nodes", str(node_count),
           "-degree", str(degree),
           "-output", topology_path]

    try:
        subprocess.run(cmd, check=True, capture_output=True)
        print(f"✓ Topology generated: {topology_path}")
        return True
    except subprocess.CalledProcessError as e:
        print(f"✗ Topology generation failed:")
        print(e.stderr.decode())
        return False


def run_simulation(protocol, node_count, msg_size, msg_count, num_chunks=None, multiplier=None, log_level="info"):
    """Run a Shadow simulation for the specified protocol."""
    print(f"\n{'='*60}")
    print(f"Running {protocol.upper()} simulation...")
    print(f"{'='*60}")

    # Use streams variants for RS and RLNC (GossipSub uses TCP, not QUIC)
    make_target = f"{protocol}-streams" if protocol in ["rlnc", "rs"] else protocol

    cmd = ["make", make_target,
           f"NODE_COUNT={node_count}",
           f"MSG_SIZE={msg_size}",
           f"MSG_COUNT={msg_count}",
           f"LOG_LEVEL={log_level}"]

    if num_chunks and protocol in ["rlnc", "rs"]:
        cmd.append(f"NUM_CHUNKS={num_chunks}")

    if multiplier and protocol in ["rlnc", "rs"]:
        cmd.append(f"MULTIPLIER={multiplier}")

    try:
        result = subprocess.run(cmd, check=True, capture_output=True, text=True)
        print(f"✓ {protocol.upper()} simulation completed successfully")
        return True
    except subprocess.CalledProcessError as e:
        print(f"✗ {protocol.upper()} simulation failed:")
        print(e.stderr)
        return False


def parse_shadow_logs(protocol, node_count, msg_count):
    """
    Parse Shadow logs to extract message arrival times.

    Returns a dict: {message_id: {node_id: arrival_time_ns}}
    """
    log_dir = f"{protocol}/shadow.data/hosts"

    if not os.path.exists(log_dir):
        print(f"Warning: Log directory not found: {log_dir}")
        return {}

    # Store arrival times: message_id -> node_id -> timestamp (nanoseconds)
    message_arrivals = defaultdict(dict)

    # Parse logs for each node
    for node_id in range(node_count):
        node_name = f"node{node_id}"
        node_dir = os.path.join(log_dir, node_name)

        if not os.path.exists(node_dir):
            print(f"Warning: Node directory not found: {node_dir}")
            continue

        # Find log file - Shadow creates files with PID suffix like "protocol-sim.1000.stderr"
        # Logs are written to stderr, not stdout
        log_files = [f for f in os.listdir(node_dir) if f.startswith(f"{protocol}-sim") and f.endswith(".stderr")]

        if not log_files:
            print(f"Warning: No log files found in {node_dir}")
            continue

        # Use the first matching log file
        log_file = os.path.join(node_dir, log_files[0])

        with open(log_file, 'r') as f:
            for line in f:
                # Look for message received/reconstructed patterns
                # GossipSub: "Received message"
                # RLNC/RS: "Successfully reconstructed message" or "Received message"

                if "Received message" in line or "Successfully reconstructed message" in line:
                    # Extract timestamp from Shadow log format
                    # Format: [timestamp] log message
                    timestamp_match = re.search(r'\[(\d+):(\d+):(\d+)\.(\d+)\]', line)
                    if not timestamp_match:
                        # Try alternate format: YYYY/MM/DD HH:MM:SS.microseconds
                        timestamp_match = re.search(r'(\d{4})/(\d{2})/(\d{2}) (\d{2}):(\d{2}):(\d{2})\.(\d+)', line)
                        if timestamp_match:
                            # Convert to nanoseconds (Shadow virtual time)
                            hour = int(timestamp_match.group(4))
                            minute = int(timestamp_match.group(5))
                            second = int(timestamp_match.group(6))
                            micro = int(timestamp_match.group(7))
                            timestamp_ns = (hour * 3600 + minute * 60 + second) * 1_000_000_000 + micro * 1000
                        else:
                            continue
                    else:
                        # Format: [HH:MM:SS.microseconds]
                        hour = int(timestamp_match.group(1))
                        minute = int(timestamp_match.group(2))
                        second = int(timestamp_match.group(3))
                        micro = int(timestamp_match.group(4))
                        timestamp_ns = (hour * 3600 + minute * 60 + second) * 1_000_000_000 + micro * 1000

                    # Extract message identifier from message content
                    # Look for patterns like "Message-0", "RSMsg-0", "RLNCMsg-0"
                    msg_match = re.search(r'(Message|RSMsg|RLNCMsg)-(\d+)', line)
                    if msg_match:
                        msg_id = int(msg_match.group(2))
                        message_arrivals[msg_id][node_id] = timestamp_ns

    return message_arrivals


def calculate_message_latencies(message_arrivals):
    """
    Calculate latency for each message delivery.

    Latency = arrival_time - first_arrival_time (for that message)
    Returns a list of latencies in milliseconds.
    """
    latencies = []

    for msg_id, nodes in message_arrivals.items():
        if not nodes:
            continue

        # Find the earliest arrival time (when message was first sent/received)
        min_time = min(nodes.values())

        # Calculate latency for each node
        for node_id, arrival_time in nodes.items():
            latency_ns = arrival_time - min_time
            latency_ms = latency_ns / 1_000_000.0
            latencies.append(latency_ms)

    return latencies


def compute_cdf(data):
    """Compute CDF from data points."""
    if not data:
        return [], []

    sorted_data = np.sort(data)
    cdf = np.arange(1, len(sorted_data) + 1) / len(sorted_data)
    return sorted_data, cdf


def plot_cdfs(gossipsub_latencies, rs_latencies, rlnc_latencies, output_file, msg_size, num_chunks):
    """Plot CDF comparison of all three protocols."""
    plt.figure(figsize=(10, 6))

    # Compute CDFs
    gs_x, gs_y = compute_cdf(gossipsub_latencies)
    rs_x, rs_y = compute_cdf(rs_latencies)
    rlnc_x, rlnc_y = compute_cdf(rlnc_latencies)

    # Plot CDFs
    # GossipSub as baseline (dashed line, neutral color)
    plt.plot(gs_x, gs_y, '--', linewidth=2.5, label='GossipSub (baseline)', color='gray', alpha=0.8)

    # RS and RLNC as improvements (solid lines, distinct colors)
    plt.plot(rs_x, rs_y, '-', linewidth=2, label='Reed-Solomon', color='#2E86AB', marker='o',
             markevery=max(1, len(rs_x)//10), markersize=6)
    plt.plot(rlnc_x, rlnc_y, '-', linewidth=2, label='RLNC', color='#A23B72', marker='s',
             markevery=max(1, len(rlnc_x)//10), markersize=6)

    plt.xlabel('Message Arrival Latency (ms)', fontsize=12)
    plt.ylabel('CDF', fontsize=12)
    plt.title(f'Message Arrival Time Distribution\n(Message Size: {msg_size}B, Chunks: {num_chunks})',
              fontsize=14, fontweight='bold')
    plt.grid(True, alpha=0.3, linestyle='--')
    plt.legend(loc='lower right', fontsize=11, framealpha=0.9)

    # Set reasonable axis limits
    plt.xlim(left=0)
    plt.ylim(0, 1.05)

    plt.tight_layout()
    plt.savefig(output_file, dpi=300, bbox_inches='tight')
    print(f"\n✓ CDF plot saved to: {output_file}")

    # Print statistics
    print(f"\n{'='*60}")
    print("Statistics Summary")
    print(f"{'='*60}")

    def print_stats(name, latencies):
        if not latencies:
            print(f"\n{name}: No data")
            return
        print(f"\n{name}:")
        print(f"  Count:      {len(latencies)}")
        print(f"  Mean:       {np.mean(latencies):.2f} ms")
        print(f"  Median:     {np.median(latencies):.2f} ms")
        print(f"  P95:        {np.percentile(latencies, 95):.2f} ms")
        print(f"  P99:        {np.percentile(latencies, 99):.2f} ms")
        print(f"  Min:        {np.min(latencies):.2f} ms")
        print(f"  Max:        {np.max(latencies):.2f} ms")

    print_stats("GossipSub (baseline)", gossipsub_latencies)
    print_stats("Reed-Solomon", rs_latencies)
    print_stats("RLNC", rlnc_latencies)
    print(f"{'='*60}\n")


def main():
    parser = argparse.ArgumentParser(
        description='Compare message arrival times across GossipSub, Reed-Solomon, and RLNC protocols.',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Compare protocols with 256-byte messages and 8 chunks
  python3 compare_protocols.py --msg-size 256 --num-chunks 8

  # Run with 20 nodes and debug logging
  python3 compare_protocols.py --msg-size 512 --num-chunks 16 --node-count 20 --log-level debug

  # Specify custom topology degree and output file
  python3 compare_protocols.py --msg-size 1024 --num-chunks 32 --degree 4 -o results/comparison.png
        """
    )

    parser.add_argument('--msg-size', type=int, required=True,
                        help='Message size in bytes')
    parser.add_argument('--num-chunks', type=int, required=True,
                        help='Number of chunks for RS and RLNC')
    parser.add_argument('--node-count', type=int, default=10,
                        help='Number of nodes (default: 10)')
    parser.add_argument('--msg-count', type=int, default=1,
                        help='Number of messages to send (default: 1)')
    parser.add_argument('--multiplier', type=int, default=4,
                        help='Multiplier for publish and forward (RS: includes min emit count) (default: 4)')
    parser.add_argument('--log-level', type=str, default='info',
                        choices=['debug', 'info', 'warn', 'error'],
                        help='Log level for simulations (default: info)')
    parser.add_argument('--degree', type=int, default=3,
                        help='Node degree for random regular topology (default: 3)')
    parser.add_argument('-o', '--output', type=str, default='protocol_comparison_cdf.png',
                        help='Output file for CDF plot (default: protocol_comparison_cdf.png)')

    args = parser.parse_args()

    print(f"""
{'='*60}
Protocol Comparison Tool
{'='*60}
Message Size:    {args.msg_size} bytes
Chunks:          {args.num_chunks}
Node Count:      {args.node_count}
Message Count:   {args.msg_count}
Topology:        Random regular (degree {args.degree})
Log Level:       {args.log_level}
Output:          {args.output}
{'='*60}
""")

    # Check if matplotlib is available
    try:
        import matplotlib.pyplot as plt
    except ImportError:
        print("Error: matplotlib is required for plotting.")
        print("Install with: pip3 install matplotlib")
        sys.exit(1)

    # Generate random regular topology
    if not generate_topology(args.node_count, args.degree):
        print("\nError: Topology generation failed. Aborting.")
        sys.exit(1)

    # Run simulations for all three protocols
    protocols = ['gossipsub', 'rs', 'rlnc']
    success = {}

    for protocol in protocols:
        success[protocol] = run_simulation(
            protocol,
            args.node_count,
            args.msg_size,
            args.msg_count,
            args.num_chunks if protocol in ['rs', 'rlnc'] else None,
            args.multiplier if protocol in ['rs', 'rlnc'] else None,
            args.log_level
        )

        if not success[protocol]:
            print(f"\nError: {protocol.upper()} simulation failed. Aborting.")
            sys.exit(1)

    # Parse logs and extract arrival times
    print(f"\n{'='*60}")
    print("Parsing simulation logs...")
    print(f"{'='*60}")

    gossipsub_arrivals = parse_shadow_logs('gossipsub', args.node_count, args.msg_count)
    rs_arrivals = parse_shadow_logs('rs', args.node_count, args.msg_count)
    rlnc_arrivals = parse_shadow_logs('rlnc', args.node_count, args.msg_count)

    print(f"✓ GossipSub: Extracted {sum(len(v) for v in gossipsub_arrivals.values())} message arrivals")
    print(f"✓ Reed-Solomon: Extracted {sum(len(v) for v in rs_arrivals.values())} message arrivals")
    print(f"✓ RLNC: Extracted {sum(len(v) for v in rlnc_arrivals.values())} message arrivals")

    # Calculate latencies
    gossipsub_latencies = calculate_message_latencies(gossipsub_arrivals)
    rs_latencies = calculate_message_latencies(rs_arrivals)
    rlnc_latencies = calculate_message_latencies(rlnc_arrivals)

    if not gossipsub_latencies and not rs_latencies and not rlnc_latencies:
        print("\nError: No message arrival data found in logs.")
        print("Make sure the simulations completed successfully and logs contain message arrival timestamps.")
        sys.exit(1)

    # Plot CDFs
    plot_cdfs(gossipsub_latencies, rs_latencies, rlnc_latencies,
              args.output, args.msg_size, args.num_chunks)

    print(f"\n{'='*60}")
    print("✓ Protocol comparison completed successfully!")
    print(f"{'='*60}\n")


if __name__ == '__main__':
    main()
