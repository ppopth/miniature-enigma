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
import time
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

    start_time = time.time()
    try:
        result = subprocess.run(cmd, check=True, capture_output=True, text=True)
        elapsed = time.time() - start_time
        print(f"✓ {protocol.upper()} simulation completed successfully in {elapsed:.2f} seconds")
        return True, elapsed
    except subprocess.CalledProcessError as e:
        elapsed = time.time() - start_time
        print(f"✗ {protocol.upper()} simulation failed after {elapsed:.2f} seconds:")
        print(e.stderr)
        return False, elapsed


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

    Latency = arrival_time - publish_time
    Publish time is assumed to be 2000/01/01 00:02:00
    Returns a list of latencies in milliseconds.
    """
    latencies = []

    # Publish time is 2000/01/01 00:02:00 (2 minutes = 120 seconds)
    publish_time_seconds = 2 * 60
    publish_time_ns = publish_time_seconds * 1_000_000_000

    for msg_id, nodes in message_arrivals.items():
        if not nodes:
            continue

        # Calculate latency for each node relative to publish time
        for node_id, arrival_time in nodes.items():
            latency_ns = arrival_time - publish_time_ns
            latency_ms = latency_ns / 1_000_000.0
            latencies.append(latency_ms)

    return latencies


def parse_chunk_statistics(protocol, node_count):
    """
    Parse chunk statistics from Shadow logs.

    Returns a dict: {node_id: [(timestamp_ns, useful_chunks, useless_chunks)]}
    """
    log_dir = f"{protocol}/shadow.data/hosts"

    if not os.path.exists(log_dir):
        print(f"Warning: Log directory not found: {log_dir}")
        return {}

    # Store chunk stats: node_id -> [(time, useful, useless)]
    node_stats = defaultdict(list)

    # Parse logs for each node
    for node_id in range(node_count):
        node_name = f"node{node_id}"
        node_dir = os.path.join(log_dir, node_name)

        if not os.path.exists(node_dir):
            continue

        # Find log file
        log_files = [f for f in os.listdir(node_dir) if f.startswith(f"{protocol}-sim") and f.endswith(".stderr")]

        if not log_files:
            continue

        log_file = os.path.join(node_dir, log_files[0])

        with open(log_file, 'r') as f:
            for line in f:
                # Look for chunk event logs: "Chunk event: Useful: X, Useless: Y"
                if "Chunk event:" in line:
                    # Extract timestamp - ISO 8601 format: 2000-01-01T00:02:00.002Z
                    timestamp_match = re.search(r'(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})\.(\d{3})', line)
                    if not timestamp_match:
                        continue

                    hour = int(timestamp_match.group(4))
                    minute = int(timestamp_match.group(5))
                    second = int(timestamp_match.group(6))
                    millisecond = int(timestamp_match.group(7))
                    timestamp_ns = (hour * 3600 + minute * 60 + second) * 1_000_000_000 + millisecond * 1_000_000

                    # Extract useful and useless chunks
                    useful_match = re.search(r'Useful: (\d+)', line)
                    useless_match = re.search(r'Useless: (\d+)', line)

                    if useful_match and useless_match:
                        useful_chunks = int(useful_match.group(1))
                        useless_chunks = int(useless_match.group(1))
                        node_stats[node_id].append((timestamp_ns, useful_chunks, useless_chunks))

    return node_stats


def compute_cdf(data):
    """Compute CDF from data points."""
    if not data:
        return [], []

    sorted_data = np.sort(data)
    cdf = np.arange(1, len(sorted_data) + 1) / len(sorted_data)
    return sorted_data, cdf


def plot_chunk_statistics(rs_stats, rlnc_stats, output_file, node_count, num_chunks, multiplier):
    """Plot time series of useful and useless chunks for RS and RLNC protocols."""
    fig, axes = plt.subplots(1, 2, figsize=(16, 6))
    fig.suptitle('Chunk Statistics Over Time (Average Per Node)', fontsize=16, fontweight='bold')

    protocols = [
        (f'RS(2k) (k={num_chunks}, D={multiplier}, routing=random)', rs_stats, '#2E86AB'),
        (f'RLNC(n=kD) (k={num_chunks}, D={multiplier}, routing=random)', rlnc_stats, '#A23B72')
    ]

    # First pass: compute all data to find the global max y-value
    all_plot_data = []
    global_max_y = 0

    for protocol_idx, (protocol_name, stats, color) in enumerate(protocols):
        if not stats:
            all_plot_data.append(None)
            continue

        # Aggregate data across all nodes
        # First, collect all unique timestamps
        all_timestamps = set()
        for node_data in stats.values():
            for t, _, _ in node_data:
                all_timestamps.add(t)

        sorted_times = sorted(all_timestamps)

        # Publish time is 2000/01/01 00:02:00 (2 minutes = 120 seconds = 120_000_000_000 ns)
        publish_time_ns = 2 * 60 * 1_000_000_000

        # Adjust timestamps to start from publish time and convert to milliseconds
        adjusted_times = [(t - publish_time_ns) / 1_000_000 for t in sorted_times]

        # For each timestamp, sum the values across all nodes
        aggregated_useful = []
        aggregated_useless = []

        for timestamp in sorted_times:
            useful_sum = 0
            useless_sum = 0

            for node_id in stats.keys():
                node_data = stats[node_id]
                # Find the closest timestamp <= current timestamp for this node
                closest_value = None
                for t, useful, useless in node_data:
                    if t <= timestamp:
                        closest_value = (useful, useless)
                    else:
                        break

                if closest_value:
                    useful_sum += closest_value[0]
                    useless_sum += closest_value[1]

            aggregated_useful.append(useful_sum)
            aggregated_useless.append(useless_sum)

        # Divide by number of nodes to get average per node
        avg_useful = [u / node_count for u in aggregated_useful]
        avg_useless = [u / node_count for u in aggregated_useless]

        # Calculate max y value (total = useful + useless)
        max_total = max([useful + useless for useful, useless in zip(avg_useful, avg_useless)]) if avg_useful else 0
        global_max_y = max(global_max_y, max_total)

        # Store data for second pass
        all_plot_data.append({
            'adjusted_times': adjusted_times,
            'avg_useful': avg_useful,
            'avg_useless': avg_useless
        })

    # Second pass: plot with uniform y-axis
    for protocol_idx, (protocol_name, stats, color) in enumerate(protocols):
        ax = axes[protocol_idx]
        plot_data = all_plot_data[protocol_idx]

        if plot_data is None:
            ax.text(0.5, 0.5, 'No data available', ha='center', va='center', transform=ax.transAxes)
            continue

        adjusted_times = plot_data['adjusted_times']
        avg_useful = plot_data['avg_useful']
        avg_useless = plot_data['avg_useless']

        # Plot stacked area chart (useful + useless)
        # Use different shades of the same color for useful (darker) and useless (lighter)
        ax.fill_between(adjusted_times, 0, avg_useful,
                       color=color, alpha=0.7, label='Useful')
        ax.fill_between(adjusted_times, avg_useful,
                       [useful + useless for useful, useless in zip(avg_useful, avg_useless)],
                       color=color, alpha=0.3, label='Useless')

        # Add lines for clearer boundaries
        useful_line = ax.plot(adjusted_times, avg_useful,
                            linewidth=1.5, color=color, alpha=0.9, label='Useful (line)')
        total_line = ax.plot(adjusted_times, [useful + useless for useful, useless in zip(avg_useful, avg_useless)],
                            linewidth=1.5, color=color, alpha=0.9, linestyle='--', label='Total (line)')

        # Configure plot
        ax.set_title(f'{protocol_name}', fontsize=14, fontweight='bold')
        ax.set_xlabel('Time (milliseconds)', fontsize=11)
        ax.set_ylabel('Chunk Count', fontsize=11)
        ax.grid(True, alpha=0.3, linestyle='--')

        # Create custom legend with better labels
        from matplotlib.patches import Patch
        from matplotlib.lines import Line2D
        legend_elements = [
            Patch(facecolor=color, alpha=0.7, label='Useful chunks'),
            Patch(facecolor=color, alpha=0.3, label='Useless chunks (stacked)'),
            Line2D([0], [0], color=color, linewidth=1.5, alpha=0.9, label='Useful count'),
            Line2D([0], [0], color=color, linewidth=1.5, alpha=0.9, linestyle='--', label='Total count')
        ]
        ax.legend(handles=legend_elements, loc='upper left', fontsize=10, framealpha=0.9)

        # Set x-axis to start from 0 (publish time) if we have data
        if adjusted_times:
            ax.set_xlim(left=min(0, min(adjusted_times)))
        else:
            ax.set_xlim(left=0)

        # Set y-axis with same scale for both plots
        ax.set_ylim(bottom=0, top=global_max_y * 1.05)  # Add 5% padding at top

    plt.tight_layout()
    plt.savefig(output_file, dpi=300, bbox_inches='tight')
    print(f"✓ Chunk statistics plot saved to: {output_file}")

    # Print summary statistics
    print(f"\n{'='*60}")
    print("Chunk Statistics Summary")
    print(f"{'='*60}")

    for protocol_name, stats, _ in protocols:
        if not stats:
            print(f"\n{protocol_name}: No data")
            continue

        all_useful = []
        all_useless = []
        for node_data in stats.values():
            if node_data:
                # Get final counts (last entry for each node)
                _, final_useful, final_useless = node_data[-1]
                all_useful.append(final_useful)
                all_useless.append(final_useless)

        if all_useful:
            print(f"\n{protocol_name}:")
            print(f"  Useful chunks (final):")
            print(f"    Mean:       {np.mean(all_useful):.1f}")
            print(f"    Min:        {np.min(all_useful)}")
            print(f"    Max:        {np.max(all_useful)}")
            print(f"  Useless chunks (final):")
            print(f"    Mean:       {np.mean(all_useless):.1f}")
            print(f"    Min:        {np.min(all_useless)}")
            print(f"    Max:        {np.max(all_useless)}")
            if sum(all_useful) > 0:
                useless_rate = sum(all_useless) / (sum(all_useful) + sum(all_useless)) * 100
                print(f"  Useless rate: {useless_rate:.2f}%")

    print(f"{'='*60}\n")


def plot_cdfs(gossipsub_latencies, rs_latencies, rlnc_latencies, output_file, msg_size, num_chunks, node_count, degree, multiplier):
    """Plot CDF comparison of all three protocols."""
    plt.figure(figsize=(10, 6))

    # Compute CDFs
    gs_x, gs_y = compute_cdf(gossipsub_latencies)
    rs_x, rs_y = compute_cdf(rs_latencies)
    rlnc_x, rlnc_y = compute_cdf(rlnc_latencies)

    # Plot CDFs
    # GossipSub as baseline (dashed line, neutral color)
    plt.plot(gs_x, gs_y, '--', linewidth=2.5, label='GossipSub (baseline, D=8)', color='gray', alpha=0.8)

    # RS and RLNC as improvements (solid lines, distinct colors, no markers)
    plt.plot(rs_x, rs_y, '-', linewidth=2, label=f'RS(2k) (k={num_chunks}, D={multiplier}, routing=random)', color='#2E86AB')
    plt.plot(rlnc_x, rlnc_y, '-', linewidth=2, label=f'RLNC(n=kD) (k={num_chunks}, D={multiplier}, routing=random)', color='#A23B72')

    plt.xlabel('Message Arrival Latency (ms)', fontsize=12)
    plt.ylabel('CDF', fontsize=12)
    plt.title(f'Message Arrival Time Distribution\n(Nodes: {node_count}, Peers: {degree}, Message Size: {msg_size}B)',
              fontsize=14, fontweight='bold')
    plt.grid(True, alpha=0.3, linestyle='--')
    plt.legend(loc='lower right', fontsize=11, framealpha=0.9)

    # Set reasonable axis limits
    # Auto-scale x-axis, but cap at 4 seconds if data exceeds that
    max_latency = max(max(gs_x, default=0), max(rs_x, default=0), max(rlnc_x, default=0))
    if max_latency <= 4000:
        # Use auto-scaling if all data fits within 4 seconds
        plt.xlim(left=0)
    else:
        # Cap at 4 seconds if data exceeds it
        plt.xlim(left=0, right=4000)
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

  # Specify custom topology degree and output files
  python3 compare_protocols.py --msg-size 1024 --num-chunks 32 --degree 4 -o results/comparison.png --chunk-stats-output results/chunks.png
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
    parser.add_argument('--chunk-stats-output', type=str, default='protocol_chunk_statistics.png',
                        help='Output file for chunk statistics plot (default: protocol_chunk_statistics.png)')

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
CDF Output:      {args.output}
Chunk Stats:     {args.chunk_stats_output}
{'='*60}
""")

    # Check if matplotlib is available
    try:
        import matplotlib.pyplot as plt
    except ImportError:
        print("Error: matplotlib is required for plotting.")
        print("Install with: pip3 install matplotlib")
        sys.exit(1)

    # Validate message size for RLNC/RS (must be divisible by num_chunks * 32)
    # Both protocols use 256-bit prime field with 32 bytes per element
    chunk_size = args.msg_size // args.num_chunks
    if chunk_size % 32 != 0:
        print(f"\nError: Invalid message size configuration!")
        print(f"  Message size ({args.msg_size}) / Num chunks ({args.num_chunks}) = {chunk_size} bytes per chunk")
        print(f"  Chunk size must be divisible by 32 (field element size)")
        print(f"\nSuggested fix:")
        print(f"  - Use MSG_SIZE that is divisible by {args.num_chunks * 32} (NUM_CHUNKS * 32)")
        print(f"  - For NUM_CHUNKS={args.num_chunks}, use MSG_SIZE >= {args.num_chunks * 32} (e.g., 256, 512, 1024, etc.)")
        sys.exit(1)

    # Generate random regular topology
    if not generate_topology(args.node_count, args.degree):
        print("\nError: Topology generation failed. Aborting.")
        sys.exit(1)

    # Run simulations for all three protocols
    protocols = ['gossipsub', 'rs', 'rlnc']
    success = {}
    timings = {}

    for protocol in protocols:
        success[protocol], timings[protocol] = run_simulation(
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

    # Print timing summary
    print(f"\n{'='*60}")
    print("Simulation Timing Summary")
    print(f"{'='*60}")
    total_time = sum(timings.values())
    for protocol in protocols:
        print(f"{protocol.upper():12s}: {timings[protocol]:6.2f} seconds")
    print(f"{'─'*60}")
    print(f"{'Total':12s}: {total_time:6.2f} seconds")
    print(f"{'='*60}")

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
              args.output, args.msg_size, args.num_chunks, args.node_count, args.degree, args.multiplier)

    # Parse and plot chunk statistics for RS and RLNC
    print(f"\n{'='*60}")
    print("Parsing chunk statistics...")
    print(f"{'='*60}")

    rs_chunk_stats = parse_chunk_statistics('rs', args.node_count)
    rlnc_chunk_stats = parse_chunk_statistics('rlnc', args.node_count)

    print(f"✓ Reed-Solomon: Extracted chunk statistics for {len(rs_chunk_stats)} nodes")
    print(f"✓ RLNC: Extracted chunk statistics for {len(rlnc_chunk_stats)} nodes")

    if rs_chunk_stats or rlnc_chunk_stats:
        plot_chunk_statistics(rs_chunk_stats, rlnc_chunk_stats,
                            args.chunk_stats_output, args.node_count, args.num_chunks, args.multiplier)
    else:
        print("\nWarning: No chunk statistics found in logs.")

    print(f"\n{'='*60}")
    print("✓ Protocol comparison completed successfully!")
    print(f"{'='*60}\n")


if __name__ == '__main__':
    main()

# Chunk Statistics Visualization
#
# The script now generates two plots:
# 1. CDF of message arrival times (existing functionality)
# 2. Stacked area chart of useful/useless chunks averaged across all nodes (new feature)
#
# The chunk statistics plot shows (1 row, 2 columns):
# - Left: Reed-Solomon stacked area chart
# - Right: RLNC stacked area chart
#
# Each chart contains:
# - Darker shaded area: Average useful chunks per node (alpha=0.7)
# - Lighter shaded area: Average useless chunks per node stacked on top (alpha=0.3)
# - Solid line: Average useful chunk count per node
# - Dashed line: Average total chunk count per node (useful + useless)
# - Legend showing all four elements
#
# This allows you to visualize:
# - Average accumulation of useful and useless chunks per node
# - Proportion of useless to useful chunks at any point in time
# - Average rate of useless chunks (duplicates, linearly dependent, post-reconstruction)
# - Side-by-side comparison of RS vs RLNC chunk efficiency
