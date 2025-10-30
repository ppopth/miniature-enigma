#!/usr/bin/env python3
"""
Test script to verify Shadow simulation results by reading log files
Usage: python3 test_results.py [node_count] [expected_messages] [protocol]
"""

import sys
import os
import glob
import re
from typing import Dict, List, Tuple

def parse_shadow_logs(node_count: int) -> Dict[int, int]:
    """Parse Shadow log files to extract message reception counts."""
    received_counts = {}

    for node_id in range(node_count):
        node_dir = f"shadow.data/hosts/node{node_id}"
        received_count = 0

        if os.path.exists(node_dir):
            # Find stderr files (Shadow logs are in stderr for Go programs)
            stderr_files = glob.glob(f"{node_dir}/*.stderr")

            if stderr_files:
                try:
                    # Use the first stderr file found
                    log_file = stderr_files[0]
                    with open(log_file, 'r') as f:
                        for line in f:
                            # Look for "Received message" log entries
                            if "Received message" in line:
                                received_count += 1
                except Exception as e:
                    print(f"Warning: Could not read {log_file}: {e}")
            else:
                print(f"Warning: No stderr files found in {node_dir}")
        else:
            print(f"Warning: Node directory not found: {node_dir}")

        received_counts[node_id] = received_count

    return received_counts

def test_message_delivery(node_count: int, expected_messages: int, protocol: str = "floodsub") -> bool:
    """Test that >99% of nodes received the expected number of messages."""
    print(f"Shadow Simulation Test Results ({protocol.upper()})")
    print("=" * 50)

    received_counts = parse_shadow_logs(node_count)

    passed_nodes = 0
    total_received = 0

    for node_id in range(node_count):
        received = received_counts.get(node_id, 0)
        total_received += received

        if received == expected_messages:
            status = "✓ PASS"
            passed_nodes += 1
        else:
            status = "✗ FAIL"

        print(f"Node {node_id:2d}: {received:2d}/{expected_messages:2d} messages {status}")

    print("-" * 40)
    print(f"Total messages received: {total_received}")
    print(f"Expected total: {node_count * expected_messages}")

    # Calculate success rate
    success_rate = (passed_nodes / node_count) * 100
    print(f"Success rate: {success_rate:.1f}% ({passed_nodes}/{node_count} nodes)")

    # Pass if >99% of nodes received all messages
    threshold = 99.0
    if success_rate > threshold:
        print(f"✓ TEST PASSED: {success_rate:.1f}% of nodes received expected messages (>{threshold}% threshold)")
        return True
    else:
        failed_nodes = [i for i in range(node_count) if received_counts.get(i, 0) != expected_messages]
        print(f"✗ TEST FAILED: Only {success_rate:.1f}% of nodes received all messages (<={threshold}% threshold)")
        print(f"Failed nodes: {failed_nodes}")
        return False

def check_shadow_data_exists() -> bool:
    """Check if Shadow simulation data directory exists."""
    if not os.path.exists("shadow.data"):
        print("Error: shadow.data directory not found. Run the simulation first.")
        return False

    if not os.path.exists("shadow.data/hosts"):
        print("Error: shadow.data/hosts directory not found. Simulation may have failed.")
        return False

    return True

def main():
    """Main function."""
    if len(sys.argv) < 3:
        print("Usage: python3 test_results.py [node_count] [expected_messages] [protocol]")
        print("Example: python3 test_results.py 10 5 floodsub")
        sys.exit(1)

    try:
        node_count = int(sys.argv[1])
        expected_messages = int(sys.argv[2])
        protocol = sys.argv[3] if len(sys.argv) > 3 else "floodsub"
    except ValueError:
        print("Error: node_count and expected_messages must be integers")
        sys.exit(1)

    if not check_shadow_data_exists():
        sys.exit(1)

    success = test_message_delivery(node_count, expected_messages, protocol)
    sys.exit(0 if success else 1)

if __name__ == "__main__":
    main()
