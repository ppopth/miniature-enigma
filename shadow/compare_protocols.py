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


def run_simulation(protocol, node_count, msg_size, msg_count, num_chunks=None, multiplier=None, log_level="info", disable_completion_signal=False, bandwidth_interval=None, bitmap_threshold=None):
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

    if disable_completion_signal and protocol in ["rlnc", "rs"]:
        cmd.append("DISABLE_COMPLETION_SIGNAL=true")

    if bandwidth_interval and protocol in ["rlnc", "rs"]:
        cmd.append(f"BANDWIDTH_INTERVAL={bandwidth_interval}")

    if bitmap_threshold is not None and protocol == "rs":
        cmd.append(f"BITMAP_THRESHOLD={bitmap_threshold}")

    # Display the command being executed
    print(f"Command: {' '.join(cmd)}")

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


def parse_bandwidth_statistics(protocol, node_count, bandwidth_interval_ms=100):
    """
    Parse bandwidth statistics from Shadow logs.

    Returns a dict: {node_id: [(timestamp_ns, send_mbps, recv_mbps)]}
    Timestamps are rounded to the nearest interval boundary.

    Args:
        protocol: Protocol name (rs, rlnc, etc.)
        node_count: Number of nodes in the simulation
        bandwidth_interval_ms: Bandwidth logging interval in milliseconds (default: 100)
    """
    log_dir = f"{protocol}/shadow.data/hosts"

    if not os.path.exists(log_dir):
        print(f"Warning: Log directory not found: {log_dir}")
        return {}

    # Store bandwidth stats: node_id -> [(time, send_mbps, recv_mbps)]
    node_stats = defaultdict(list)

    # Bandwidth logging interval in nanoseconds
    interval_ns = bandwidth_interval_ms * 1_000_000

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
                # Look for bandwidth logs: "Bandwidth: SendMbps=X.XX, RecvMbps=Y.YY"
                if "Bandwidth: SendMbps=" in line:
                    # Extract timestamp - format: YYYY/MM/DD HH:MM:SS.microseconds
                    timestamp_match = re.search(r'(\d{4})/(\d{2})/(\d{2}) (\d{2}):(\d{2}):(\d{2})\.(\d+)', line)
                    if not timestamp_match:
                        continue

                    hour = int(timestamp_match.group(4))
                    minute = int(timestamp_match.group(5))
                    second = int(timestamp_match.group(6))
                    microseconds = int(timestamp_match.group(7))
                    timestamp_ns = (hour * 3600 + minute * 60 + second) * 1_000_000_000 + microseconds * 1000

                    # Round timestamp to nearest interval boundary
                    rounded_timestamp_ns = round(timestamp_ns / interval_ns) * interval_ns

                    # Extract send and receive Mbps
                    send_match = re.search(r'SendMbps=([\d.]+)', line)
                    recv_match = re.search(r'RecvMbps=([\d.]+)', line)

                    if send_match and recv_match:
                        send_mbps = float(send_match.group(1))
                        recv_mbps = float(recv_match.group(1))
                        node_stats[node_id].append((rounded_timestamp_ns, send_mbps, recv_mbps))

    return node_stats


def parse_chunk_statistics(protocol, node_count):
    """
    Parse chunk statistics from Shadow logs.

    Returns a dict: {node_id: [(timestamp_ns, useful_chunks, useless_chunks, unused_chunks, prevented_chunks)]}
    """
    log_dir = f"{protocol}/shadow.data/hosts"

    if not os.path.exists(log_dir):
        print(f"Warning: Log directory not found: {log_dir}")
        return {}

    # Store chunk stats: node_id -> [(time, useful, useless, unused, prevented)]
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

        # Track cumulative prevented count and last known chunk values for this node
        cumulative_prevented = 0
        last_useful = 0
        last_useless = 0
        last_unused = 0

        with open(log_file, 'r') as f:
            for line in f:
                # Extract timestamp - ISO 8601 format: 2000-01-01T00:02:00.002Z
                timestamp_match = re.search(r'(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})\.(\d{3})', line)
                if not timestamp_match:
                    continue

                hour = int(timestamp_match.group(4))
                minute = int(timestamp_match.group(5))
                second = int(timestamp_match.group(6))
                millisecond = int(timestamp_match.group(7))
                timestamp_ns = (hour * 3600 + minute * 60 + second) * 1_000_000_000 + millisecond * 1_000_000

                # Look for chunk event logs: "Chunk event: Useful: X, Useless: Y, Unused: Z"
                if "Chunk event:" in line:
                    # Extract useful, useless, and unused chunks
                    useful_match = re.search(r'Useful: (\d+)', line)
                    useless_match = re.search(r'Useless: (\d+)', line)
                    unused_match = re.search(r'Unused: (\d+)', line)

                    if useful_match and useless_match and unused_match:
                        useful_chunks = int(useful_match.group(1))
                        useless_chunks = int(useless_match.group(1))
                        unused_chunks = int(unused_match.group(1))
                        # Update last known values
                        last_useful = useful_chunks
                        last_useless = useless_chunks
                        last_unused = unused_chunks
                        node_stats[node_id].append((timestamp_ns, useful_chunks, useless_chunks, unused_chunks, cumulative_prevented))

                # Look for prevented chunk logs: "Prevented chunk send for message"
                elif "Prevented chunk send for message" in line:
                    cumulative_prevented += 1
                    # Add entry with updated prevented count and last known chunk values
                    node_stats[node_id].append((timestamp_ns, last_useful, last_useless, last_unused, cumulative_prevented))

    return node_stats


def compute_cdf(data):
    """Compute CDF from data points."""
    if not data:
        return [], []

    sorted_data = np.sort(data)
    cdf = np.arange(1, len(sorted_data) + 1) / len(sorted_data)
    return sorted_data, cdf


def plot_chunk_statistics(rs_stats, rlnc_stats, output_file, node_count, num_chunks, multiplier, msg_size, degree, disable_completion_signal=False, bitmap_threshold=None):
    """Plot time series of useful, useless, and unused chunks for RS and RLNC protocols."""
    fig, axes = plt.subplots(1, 2, figsize=(16, 6))
    fig.suptitle(f'Chunk Statistics Over Time (Average Per Node)\n(Nodes: {node_count}, Peers: {degree}, Message Size: {msg_size}B)',
                 fontsize=16, fontweight='bold')

    # Add "(no completion signal)" suffix only when disabled (not when enabled, since that's default)
    signal_suffix = " (no completion signal)" if disable_completion_signal else ""

    # Add bitmap threshold to RS label if provided (display as (1-threshold)*100%)
    bitmap_suffix = f", bitmap={int((1.0 - bitmap_threshold) * 100)}%" if bitmap_threshold is not None else ""

    protocols = [
        (f'RS(2k) (k={num_chunks}, D={multiplier}, routing=random{bitmap_suffix}){signal_suffix}', rs_stats, '#2E86AB'),
        (f'RLNC(n=kD) (k={num_chunks}, D={multiplier}, routing=random){signal_suffix}', rlnc_stats, '#A23B72')
    ]

    # First pass: compute all data to find the global max y-value and x-range
    all_plot_data = []
    global_max_y = 0
    global_min_x = 0
    global_max_x = 0

    for protocol_idx, (protocol_name, stats, color) in enumerate(protocols):
        if not stats:
            all_plot_data.append(None)
            continue

        # Aggregate data across all nodes
        # First, collect all unique timestamps
        all_timestamps = set()
        for node_data in stats.values():
            for t, _, _, _, _ in node_data:
                all_timestamps.add(t)

        sorted_times = sorted(all_timestamps)

        # Publish time is 2000/01/01 00:02:00 (2 minutes = 120 seconds = 120_000_000_000 ns)
        publish_time_ns = 2 * 60 * 1_000_000_000

        # Adjust timestamps to start from publish time and convert to milliseconds
        adjusted_times = [(t - publish_time_ns) / 1_000_000 for t in sorted_times]

        # For each timestamp, sum the values across all nodes
        aggregated_useful = []
        aggregated_useless = []
        aggregated_unused = []
        aggregated_prevented = []

        for timestamp in sorted_times:
            useful_sum = 0
            useless_sum = 0
            unused_sum = 0
            prevented_sum = 0

            for node_id in stats.keys():
                node_data = stats[node_id]
                # Find the closest timestamp <= current timestamp for this node
                closest_value = None
                for t, useful, useless, unused, prevented in node_data:
                    if t <= timestamp:
                        closest_value = (useful, useless, unused, prevented)
                    else:
                        break

                if closest_value:
                    useful_sum += closest_value[0]
                    useless_sum += closest_value[1]
                    unused_sum += closest_value[2]
                    prevented_sum += closest_value[3]

            aggregated_useful.append(useful_sum)
            aggregated_useless.append(useless_sum)
            aggregated_unused.append(unused_sum)
            aggregated_prevented.append(prevented_sum)

        # Divide by number of nodes to get average per node
        avg_useful = [u / node_count for u in aggregated_useful]
        avg_useless = [u / node_count for u in aggregated_useless]
        avg_unused = [u / node_count for u in aggregated_unused]
        avg_prevented = [p / node_count for p in aggregated_prevented]

        # Calculate max y value (total = useful + useless + unused + prevented)
        max_total = max([useful + useless + unused + prevented for useful, useless, unused, prevented in zip(avg_useful, avg_useless, avg_unused, avg_prevented)]) if avg_useful else 0
        global_max_y = max(global_max_y, max_total)

        # Calculate x-axis range
        if adjusted_times:
            global_min_x = min(global_min_x, min(adjusted_times))
            global_max_x = max(global_max_x, max(adjusted_times))

        # Store data for second pass
        all_plot_data.append({
            'adjusted_times': adjusted_times,
            'avg_useful': avg_useful,
            'avg_useless': avg_useless,
            'avg_unused': avg_unused,
            'avg_prevented': avg_prevented
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
        avg_unused = plot_data['avg_unused']
        avg_prevented = plot_data['avg_prevented']

        # Plot stacked area chart (useful + useless + unused + prevented)
        # Use different shades: useful (darkest), useless (medium), unused (lighter), prevented (lightest)
        ax.fill_between(adjusted_times, 0, avg_useful,
                       color=color, alpha=0.7, label='Useful')
        ax.fill_between(adjusted_times, avg_useful,
                       [useful + useless for useful, useless in zip(avg_useful, avg_useless)],
                       color='orange', alpha=0.5, label='Useless')
        ax.fill_between(adjusted_times,
                       [useful + useless for useful, useless in zip(avg_useful, avg_useless)],
                       [useful + useless + unused for useful, useless, unused in zip(avg_useful, avg_useless, avg_unused)],
                       color=color, alpha=0.3, label='Unused')
        ax.fill_between(adjusted_times,
                       [useful + useless + unused for useful, useless, unused in zip(avg_useful, avg_useless, avg_unused)],
                       [useful + useless + unused + prevented for useful, useless, unused, prevented in zip(avg_useful, avg_useless, avg_unused, avg_prevented)],
                       color='red', alpha=0.2, label='Prevented')

        # Add lines for clearer boundaries
        useful_line = ax.plot(adjusted_times, avg_useful,
                            linewidth=1.5, color=color, alpha=0.9)
        useless_line = ax.plot(adjusted_times, [useful + useless for useful, useless in zip(avg_useful, avg_useless)],
                            linewidth=1.5, color='orange', alpha=0.9)
        unused_line = ax.plot(adjusted_times, [useful + useless + unused for useful, useless, unused in zip(avg_useful, avg_useless, avg_unused)],
                            linewidth=1.5, color=color, alpha=0.9)
        total_line = ax.plot(adjusted_times, [useful + useless + unused + prevented for useful, useless, unused, prevented in zip(avg_useful, avg_useless, avg_unused, avg_prevented)],
                            linewidth=1.5, color='red', alpha=0.9, linestyle='--')

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
            Patch(facecolor='orange', alpha=0.5, label='Useless chunks (stacked)'),
            Patch(facecolor=color, alpha=0.3, label='Unused chunks (stacked)'),
            Patch(facecolor='red', alpha=0.2, label='Prevented chunks (stacked)'),
            Line2D([0], [0], color=color, linewidth=1.5, alpha=0.9, label='Useful count'),
            Line2D([0], [0], color='orange', linewidth=1.5, alpha=0.9, label='Useful+Useless count'),
            Line2D([0], [0], color=color, linewidth=1.5, alpha=0.9, label='Useful+Useless+Unused count'),
            Line2D([0], [0], color='red', linewidth=1.5, alpha=0.9, linestyle='--', label='Total count (including prevented)')
        ]
        ax.legend(handles=legend_elements, loc='upper left', fontsize=8, framealpha=0.9)

        # Set x-axis and y-axis with same scale for both plots
        ax.set_xlim(left=min(0, global_min_x), right=global_max_x * 1.02)  # Add 2% padding at right
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
        all_unused = []
        all_prevented = []
        for node_data in stats.values():
            if node_data:
                # Get final counts (last entry for each node)
                _, final_useful, final_useless, final_unused, final_prevented = node_data[-1]
                all_useful.append(final_useful)
                all_useless.append(final_useless)
                all_unused.append(final_unused)
                all_prevented.append(final_prevented)

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
            print(f"  Unused chunks (final):")
            print(f"    Mean:       {np.mean(all_unused):.1f}")
            print(f"    Min:        {np.min(all_unused)}")
            print(f"    Max:        {np.max(all_unused)}")
            print(f"  Prevented chunks (final):")
            print(f"    Mean:       {np.mean(all_prevented):.1f}")
            print(f"    Min:        {np.min(all_prevented)}")
            print(f"    Max:        {np.max(all_prevented)}")
            # Calculate total received chunks (not including prevented)
            total_received = sum(all_useful) + sum(all_useless) + sum(all_unused)
            # Calculate total that would have been sent (including prevented)
            total_with_prevented = total_received + sum(all_prevented)
            if total_with_prevented > 0:
                useless_rate = sum(all_useless) / total_received * 100 if total_received > 0 else 0
                unused_rate = sum(all_unused) / total_received * 100 if total_received > 0 else 0
                prevented_rate = sum(all_prevented) / total_with_prevented * 100
                print(f"  Useless rate: {useless_rate:.2f}% (of received chunks)")
                print(f"  Unused rate: {unused_rate:.2f}% (of received chunks)")
                print(f"  Prevented rate: {prevented_rate:.2f}% (of total that would have been sent)")

    print(f"{'='*60}\n")


def plot_bandwidth_statistics(rs_stats, rlnc_stats, output_file, node_count, num_chunks, multiplier, msg_size, degree, disable_completion_signal=False, bitmap_threshold=None):
    """Plot time series of send and receive bandwidth (Mbps) for RS and RLNC protocols."""
    fig, axes = plt.subplots(2, 2, figsize=(16, 12))
    fig.suptitle(f'Network Bandwidth Over Time (Average Per Node)\n(Nodes: {node_count}, Peers: {degree}, Message Size: {msg_size}B)',
                 fontsize=16, fontweight='bold')

    # Add "(no completion signal)" suffix only when disabled (not when enabled, since that's default)
    signal_suffix = " (no completion signal)" if disable_completion_signal else ""

    # Add bitmap threshold to RS label if provided (display as (1-threshold)*100%)
    bitmap_suffix = f", bitmap={int((1.0 - bitmap_threshold) * 100)}%" if bitmap_threshold is not None else ""

    protocols = [
        (f'RS(2k) (k={num_chunks}, D={multiplier}, routing=random{bitmap_suffix}){signal_suffix}', rs_stats, '#2E86AB'),
        (f'RLNC(n=kD) (k={num_chunks}, D={multiplier}, routing=random){signal_suffix}', rlnc_stats, '#A23B72')
    ]

    # Publish time is 2000/01/01 00:02:00 (2 minutes = 120 seconds = 120_000_000_000 ns)
    publish_time_ns = 2 * 60 * 1_000_000_000

    # First pass: compute all data to find global max x and y values
    all_plot_data = []
    global_max_x = 0
    global_max_y = 0

    for protocol_idx, (protocol_name, stats, color) in enumerate(protocols):
        if not stats:
            all_plot_data.append(None)
            continue

        # Aggregate data across all nodes efficiently
        # Collect all data points from all nodes and group by timestamp
        timestamp_data = defaultdict(lambda: {'send': [], 'recv': []})

        for node_data in stats.values():
            for t, send, recv in node_data:
                timestamp_data[t]['send'].append(send)
                timestamp_data[t]['recv'].append(recv)

        # Sort timestamps and compute averages
        sorted_times = sorted(timestamp_data.keys())
        adjusted_times = [(t - publish_time_ns) / 1_000_000 for t in sorted_times]

        avg_send_mbps = [np.mean(timestamp_data[t]['send']) for t in sorted_times]
        avg_recv_mbps = [np.mean(timestamp_data[t]['recv']) for t in sorted_times]

        # Find the last non-zero timestamp for this protocol
        last_nonzero_idx = len(adjusted_times) - 1
        for i in range(len(adjusted_times) - 1, -1, -1):
            if avg_send_mbps[i] > 0 or avg_recv_mbps[i] > 0:
                last_nonzero_idx = i
                break

        # Calculate global max values
        if adjusted_times and last_nonzero_idx >= 0:
            global_max_x = max(global_max_x, adjusted_times[last_nonzero_idx])
        if avg_send_mbps:
            global_max_y = max(global_max_y, max(avg_send_mbps))
        if avg_recv_mbps:
            global_max_y = max(global_max_y, max(avg_recv_mbps))

        # Store data for second pass
        all_plot_data.append({
            'adjusted_times': adjusted_times,
            'avg_send_mbps': avg_send_mbps,
            'avg_recv_mbps': avg_recv_mbps
        })

    # Second pass: plot with uniform x and y axis scales
    for protocol_idx, (protocol_name, stats, color) in enumerate(protocols):
        plot_data = all_plot_data[protocol_idx]

        if plot_data is None:
            for ax in axes[:, protocol_idx]:
                ax.text(0.5, 0.5, 'No data available', ha='center', va='center', transform=ax.transAxes)
            continue

        adjusted_times = plot_data['adjusted_times']
        avg_send_mbps = plot_data['avg_send_mbps']
        avg_recv_mbps = plot_data['avg_recv_mbps']

        # Plot send bandwidth (top row)
        ax_send = axes[0, protocol_idx]
        ax_send.plot(adjusted_times, avg_send_mbps, linewidth=2, color=color, alpha=0.8)
        ax_send.fill_between(adjusted_times, 0, avg_send_mbps, color=color, alpha=0.3)
        ax_send.set_title(f'{protocol_name}\nSend Bandwidth', fontsize=12, fontweight='bold')
        ax_send.set_xlabel('Time (milliseconds)', fontsize=10)
        ax_send.set_ylabel('Bandwidth (Mbps)', fontsize=10)
        ax_send.grid(True, alpha=0.3, linestyle='--')
        ax_send.set_xlim(left=0, right=global_max_x * 1.02)  # Add 2% padding
        ax_send.set_ylim(bottom=0, top=global_max_y * 1.05)  # Add 5% padding

        # Plot receive bandwidth (bottom row)
        ax_recv = axes[1, protocol_idx]
        ax_recv.plot(adjusted_times, avg_recv_mbps, linewidth=2, color=color, alpha=0.8)
        ax_recv.fill_between(adjusted_times, 0, avg_recv_mbps, color=color, alpha=0.3)
        ax_recv.set_title(f'{protocol_name}\nReceive Bandwidth', fontsize=12, fontweight='bold')
        ax_recv.set_xlabel('Time (milliseconds)', fontsize=10)
        ax_recv.set_ylabel('Bandwidth (Mbps)', fontsize=10)
        ax_recv.grid(True, alpha=0.3, linestyle='--')
        ax_recv.set_xlim(left=0, right=global_max_x * 1.02)  # Add 2% padding
        ax_recv.set_ylim(bottom=0, top=global_max_y * 1.05)  # Add 5% padding

    plt.tight_layout()
    plt.savefig(output_file, dpi=300, bbox_inches='tight')
    print(f"✓ Bandwidth statistics plot saved to: {output_file}")

    # Print summary statistics
    print(f"\n{'='*60}")
    print("Bandwidth Statistics Summary")
    print(f"{'='*60}")

    for protocol_name, stats, _ in protocols:
        if not stats:
            print(f"\n{protocol_name}: No data")
            continue

        all_send = []
        all_recv = []
        for node_data in stats.values():
            for _, send, recv in node_data:
                all_send.append(send)
                all_recv.append(recv)

        if all_send:
            print(f"\n{protocol_name}:")
            print(f"  Send bandwidth (Mbps):")
            print(f"    Mean:       {np.mean(all_send):.2f}")
            print(f"    Median:     {np.median(all_send):.2f}")
            print(f"    P95:        {np.percentile(all_send, 95):.2f}")
            print(f"    Max:        {np.max(all_send):.2f}")
            print(f"  Receive bandwidth (Mbps):")
            print(f"    Mean:       {np.mean(all_recv):.2f}")
            print(f"    Median:     {np.median(all_recv):.2f}")
            print(f"    P95:        {np.percentile(all_recv, 95):.2f}")
            print(f"    Max:        {np.max(all_recv):.2f}")

    print(f"{'='*60}\n")


def plot_cdfs(gossipsub_latencies, rs_latencies_by_threshold, rlnc_latencies, output_file, msg_size, num_chunks, node_count, degree, multiplier, disable_completion_signal=False):
    """Plot CDF comparison with multiple RS bitmap thresholds."""
    plt.figure(figsize=(12, 6))

    # Compute CDFs
    gs_x, gs_y = compute_cdf(gossipsub_latencies)
    rlnc_x, rlnc_y = compute_cdf(rlnc_latencies)

    # Add "(no completion signal)" suffix only when disabled (not when enabled, since that's default)
    signal_suffix = " (no completion signal)" if disable_completion_signal else ""

    # Plot CDFs
    # GossipSub as baseline (dashed line, neutral color)
    plt.plot(gs_x, gs_y, '--', linewidth=2.5, label='GossipSub (baseline, D=8)', color='gray', alpha=0.8)

    # RLNC (solid line)
    plt.plot(rlnc_x, rlnc_y, '-', linewidth=2, label=f'RLNC(n=kD) (k={num_chunks}, D={multiplier}, routing=random){signal_suffix}', color='#A23B72')

    # RS with different bitmap thresholds (solid lines, different shades of blue)
    # Display as (1-threshold)*100% (when threshold=0.3, display as 70% meaning bitmap starts at 70% received)
    rs_colors = ['#0d3b66', '#1a5276', '#2E86AB', '#5dade2']  # Very dark blue, dark blue, medium blue, light blue
    for i, (threshold, rs_latencies) in enumerate(sorted(rs_latencies_by_threshold.items())):
        rs_x, rs_y = compute_cdf(rs_latencies)
        plt.plot(rs_x, rs_y, '-', linewidth=2,
                label=f'RS(2k) bitmap={int((1.0 - threshold) * 100)}% (k={num_chunks}, D={multiplier}){signal_suffix}',
                color=rs_colors[i % len(rs_colors)])

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
    print_stats("RLNC", rlnc_latencies)
    for threshold in sorted(rs_latencies_by_threshold.keys()):
        print_stats(f"Reed-Solomon (bitmap={int((1.0 - threshold) * 100)}%)", rs_latencies_by_threshold[threshold])
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

  # Skip simulations and use existing results for plotting
  python3 compare_protocols.py --msg-size 256 --num-chunks 8 --skip-simulations
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
    parser.add_argument('--bandwidth-output', type=str, default='protocol_bandwidth.png',
                        help='Output file for bandwidth plot (default: protocol_bandwidth.png)')
    parser.add_argument('--skip-simulations', action='store_true',
                        help='Skip running simulations and use existing results for plotting')
    parser.add_argument('--disable-completion-signal', action='store_true',
                        help='Disable completion signals for RS and RLNC protocols (default: false, signals enabled)')
    parser.add_argument('--bandwidth-interval', type=int, default=100,
                        help='Bandwidth logging interval in milliseconds (default: 100)')

    args = parser.parse_args()

    # Hardcoded bitmap thresholds for RS protocol testing
    bitmap_thresholds = [0.0, 0.2, 0.3, 0.5]

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
Bandwidth Int:   {args.bandwidth_interval}ms
CDF Output:      {args.output}
Chunk Stats:     {args.chunk_stats_output}
Bandwidth:       {args.bandwidth_output}
{'='*60}
""")

    # Check if matplotlib is available
    try:
        import matplotlib.pyplot as plt
    except ImportError:
        print("Error: matplotlib is required for plotting.")
        print("Install with: pip3 install matplotlib")
        sys.exit(1)

    if not args.skip_simulations:
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

        # Run simulations for all protocols
        # For RS, test multiple bitmap thresholds
        protocols = ['gossipsub', 'rlnc']
        success = {}
        timings = {}

        # Run gossipsub and rlnc once
        for protocol in protocols:
            success[protocol], timings[protocol] = run_simulation(
                protocol,
                args.node_count,
                args.msg_size,
                args.msg_count,
                args.num_chunks if protocol in ['rs', 'rlnc'] else None,
                args.multiplier if protocol in ['rs', 'rlnc'] else None,
                args.log_level,
                args.disable_completion_signal,
                args.bandwidth_interval,
                None
            )

            if not success[protocol]:
                print(f"\nError: {protocol.upper()} simulation failed. Aborting.")
                sys.exit(1)

        # Run RS with different bitmap thresholds
        # Rename shadow.data after each run to preserve results
        for threshold in bitmap_thresholds:
            protocol_name = f'rs_threshold_{threshold}'
            success[protocol_name], timings[protocol_name] = run_simulation(
                'rs',
                args.node_count,
                args.msg_size,
                args.msg_count,
                args.num_chunks,
                args.multiplier,
                args.log_level,
                args.disable_completion_signal,
                args.bandwidth_interval,
                threshold
            )

            if not success[protocol_name]:
                print(f"\nError: RS simulation with threshold {threshold} failed. Aborting.")
                sys.exit(1)

            # Rename shadow.data directory to preserve results for this threshold
            import shutil
            shadow_data_src = 'rs/shadow.data'
            shadow_data_dst = f'rs/shadow.data.threshold_{threshold}'
            if os.path.exists(shadow_data_dst):
                shutil.rmtree(shadow_data_dst)
            if os.path.exists(shadow_data_src):
                shutil.move(shadow_data_src, shadow_data_dst)
                print(f"✓ Saved RS results for threshold {threshold} to {shadow_data_dst}")

        # Print timing summary
        print(f"\n{'='*60}")
        print("Simulation Timing Summary")
        print(f"{'='*60}")
        total_time = sum(timings.values())
        for protocol in ['gossipsub', 'rlnc']:
            print(f"{protocol.upper():20s}: {timings[protocol]:6.2f} seconds")
        for threshold in bitmap_thresholds:
            protocol_name = f'rs_threshold_{threshold}'
            print(f"RS (threshold={threshold}):  {timings[protocol_name]:6.2f} seconds")
        print(f"{'─'*60}")
        print(f"{'Total':20s}: {total_time:6.2f} seconds")
        print(f"{'='*60}")
    else:
        print(f"\n{'='*60}")
        print("Skipping simulations - using existing results")
        print(f"{'='*60}")

    # Parse logs and extract arrival times
    print(f"\n{'='*60}")
    print("Parsing simulation logs...")
    print(f"{'='*60}")

    gossipsub_arrivals = parse_shadow_logs('gossipsub', args.node_count, args.msg_count)
    rlnc_arrivals = parse_shadow_logs('rlnc', args.node_count, args.msg_count)

    print(f"✓ GossipSub: Extracted {sum(len(v) for v in gossipsub_arrivals.values())} message arrivals")
    print(f"✓ RLNC: Extracted {sum(len(v) for v in rlnc_arrivals.values())} message arrivals")

    # Parse RS logs for each threshold from renamed directories
    rs_arrivals_by_threshold = {}
    for threshold in bitmap_thresholds:
        # Temporarily rename directory back to parse logs
        import shutil
        shadow_data_src = f'rs/shadow.data.threshold_{threshold}'
        shadow_data_temp = 'rs/shadow.data'

        if os.path.exists(shadow_data_temp):
            shutil.rmtree(shadow_data_temp)
        if os.path.exists(shadow_data_src):
            shutil.copytree(shadow_data_src, shadow_data_temp)

        rs_arrivals = parse_shadow_logs('rs', args.node_count, args.msg_count)
        rs_arrivals_by_threshold[threshold] = rs_arrivals
        print(f"✓ Reed-Solomon (threshold={threshold}): Extracted {sum(len(v) for v in rs_arrivals.values())} message arrivals")

        # Clean up temp directory
        if os.path.exists(shadow_data_temp):
            shutil.rmtree(shadow_data_temp)

    # Calculate latencies
    gossipsub_latencies = calculate_message_latencies(gossipsub_arrivals)
    rlnc_latencies = calculate_message_latencies(rlnc_arrivals)
    rs_latencies_by_threshold = {threshold: calculate_message_latencies(arrivals)
                                  for threshold, arrivals in rs_arrivals_by_threshold.items()}

    has_data = gossipsub_latencies or rlnc_latencies or any(rs_latencies_by_threshold.values())
    if not has_data:
        print("\nError: No message arrival data found in logs.")
        print("Make sure the simulations completed successfully and logs contain message arrival timestamps.")
        sys.exit(1)

    # Plot CDFs
    plot_cdfs(gossipsub_latencies, rs_latencies_by_threshold, rlnc_latencies,
              args.output, args.msg_size, args.num_chunks, args.node_count, args.degree, args.multiplier,
              args.disable_completion_signal)

    # Parse and plot chunk statistics for RS (each threshold) and RLNC
    print(f"\n{'='*60}")
    print("Parsing chunk statistics...")
    print(f"{'='*60}")

    rlnc_chunk_stats = parse_chunk_statistics('rlnc', args.node_count)
    print(f"✓ RLNC: Extracted chunk statistics for {len(rlnc_chunk_stats)} nodes")

    # Parse chunk stats for each RS threshold
    rs_chunk_stats_by_threshold = {}
    for threshold in bitmap_thresholds:
        # Temporarily copy directory back to parse
        import shutil
        shadow_data_src = f'rs/shadow.data.threshold_{threshold}'
        shadow_data_temp = 'rs/shadow.data'

        if os.path.exists(shadow_data_temp):
            shutil.rmtree(shadow_data_temp)
        if os.path.exists(shadow_data_src):
            shutil.copytree(shadow_data_src, shadow_data_temp)

        rs_chunk_stats = parse_chunk_statistics('rs', args.node_count)
        rs_chunk_stats_by_threshold[threshold] = rs_chunk_stats
        print(f"✓ Reed-Solomon (threshold={threshold}): Extracted chunk statistics for {len(rs_chunk_stats)} nodes")

        # Clean up temp directory
        if os.path.exists(shadow_data_temp):
            shutil.rmtree(shadow_data_temp)

    # Plot chunk statistics for each threshold separately
    for threshold in bitmap_thresholds:
        rs_chunk_stats = rs_chunk_stats_by_threshold[threshold]
        if rs_chunk_stats or rlnc_chunk_stats:
            output_file = args.chunk_stats_output.replace('.png', f'_threshold_{threshold}.png')
            plot_chunk_statistics(rs_chunk_stats, rlnc_chunk_stats,
                                output_file, args.node_count, args.num_chunks, args.multiplier,
                                args.msg_size, args.degree, args.disable_completion_signal, threshold)
        else:
            print(f"\nWarning: No chunk statistics found for threshold {threshold}.")

    # Parse and plot bandwidth statistics for RS (each threshold) and RLNC
    print(f"\n{'='*60}")
    print("Parsing bandwidth statistics...")
    print(f"{'='*60}")

    rlnc_bandwidth_stats = parse_bandwidth_statistics('rlnc', args.node_count, args.bandwidth_interval)
    print(f"✓ RLNC: Extracted bandwidth statistics for {len(rlnc_bandwidth_stats)} nodes")

    # Parse bandwidth stats for each RS threshold
    rs_bandwidth_stats_by_threshold = {}
    for threshold in bitmap_thresholds:
        # Temporarily copy directory back to parse
        import shutil
        shadow_data_src = f'rs/shadow.data.threshold_{threshold}'
        shadow_data_temp = 'rs/shadow.data'

        if os.path.exists(shadow_data_temp):
            shutil.rmtree(shadow_data_temp)
        if os.path.exists(shadow_data_src):
            shutil.copytree(shadow_data_src, shadow_data_temp)

        rs_bandwidth_stats = parse_bandwidth_statistics('rs', args.node_count, args.bandwidth_interval)
        rs_bandwidth_stats_by_threshold[threshold] = rs_bandwidth_stats
        print(f"✓ Reed-Solomon (threshold={threshold}): Extracted bandwidth statistics for {len(rs_bandwidth_stats)} nodes")

        # Clean up temp directory
        if os.path.exists(shadow_data_temp):
            shutil.rmtree(shadow_data_temp)

    # Plot bandwidth statistics for each threshold separately
    for threshold in bitmap_thresholds:
        rs_bandwidth_stats = rs_bandwidth_stats_by_threshold[threshold]
        if rs_bandwidth_stats or rlnc_bandwidth_stats:
            output_file = args.bandwidth_output.replace('.png', f'_threshold_{threshold}.png')
            plot_bandwidth_statistics(rs_bandwidth_stats, rlnc_bandwidth_stats,
                                     output_file, args.node_count, args.num_chunks, args.multiplier,
                                     args.msg_size, args.degree, args.disable_completion_signal, threshold)
        else:
            print(f"\nWarning: No bandwidth statistics found for threshold {threshold}.")

    print(f"\n{'='*60}")
    print("✓ Protocol comparison completed successfully!")
    print(f"{'='*60}\n")


if __name__ == '__main__':
    main()

# Chunk Statistics Visualization
#
# The script now generates two plots:
# 1. CDF of message arrival times (existing functionality)
# 2. Stacked area chart of useful/useless/unused chunks averaged across all nodes (new feature)
#
# The chunk statistics plot shows (1 row, 2 columns):
# - Left: Reed-Solomon stacked area chart
# - Right: RLNC stacked area chart
#
# Each chart contains:
# - Darkest shaded area: Average useful chunks per node (protocol color, alpha=0.7)
# - Medium shaded area: Average useless chunks per node stacked on top (orange, alpha=0.5)
# - Lightest shaded area: Average unused chunks per node stacked on top (protocol color, alpha=0.3)
# - Solid line (protocol color): Average useful chunk count per node
# - Solid line (orange): Average useful+useless chunk count per node
# - Dashed line (protocol color): Average total chunk count per node (useful + useless + unused)
# - Legend showing all six elements
#
# Chunk semantics:
# - Useful chunks: Chunks received BEFORE reconstruction is possible (linearly independent for RLNC, valid for RS)
# - Useless chunks: Invalid/duplicate/failed chunks received BEFORE reconstruction is possible
# - Unused chunks: Any chunks (valid or invalid) received AFTER reconstruction is already possible
#
# This allows you to visualize:
# - Average accumulation of useful, useless, and unused chunks per node
# - Proportion of each chunk type at any point in time
# - Average rate of useless chunks (duplicates, linearly dependent, verification failures before reconstruction)
# - Average rate of unused chunks (all chunks after reconstruction is possible)
# - Side-by-side comparison of RS vs RLNC chunk efficiency
