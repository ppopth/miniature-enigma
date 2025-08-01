#!/bin/bash

# Simple RLNC Network Test
# Tests that 3 nodes can start and connect

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { echo -e "${BLUE}[TEST]${NC} $1"; }
success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Build if needed
if [ ! -f "./rlnc-network" ]; then
    log "Building RLNC network..."
    go build
fi

log "Testing RLNC network with 3 nodes..."

# Start first node (hub) - keep alive with delayed quit
log "Starting node1 (hub)..."
(sleep 20; echo "quit") | ./rlnc-network -l 8001 -id node1 > node1.log 2>&1 &
PID1=$!

sleep 3

# Start second node connecting to first
log "Starting node2..."
(sleep 20; echo "quit") | ./rlnc-network -l 8002 -c 127.0.0.1:8001 -id node2 > node2.log 2>&1 &
PID2=$!

sleep 3

# Start third node connecting to both
log "Starting node3..."
(sleep 20; echo "quit") | ./rlnc-network -l 8003 -c 127.0.0.1:8001,127.0.0.1:8002 -id node3 > node3.log 2>&1 &
PID3=$!

# Cleanup function
cleanup() {
    log "Cleaning up..."
    kill $PID1 $PID2 $PID3 2>/dev/null || true
    wait $PID1 $PID2 $PID3 2>/dev/null || true
    rm -f node1.log node2.log node3.log
}

trap cleanup EXIT

# Wait for nodes to initialize and connect
log "Waiting for nodes to initialize and connect..."
sleep 5

# Check if all nodes are still running
if kill -0 $PID1 $PID2 $PID3 2>/dev/null; then
    # Check logs for successful startup indicators
    success_count=0

    if grep -q "Using prime field GF(4294967311)" node1.log; then
        success_count=$((success_count + 1))
    fi
    if grep -q "Using prime field GF(4294967311)" node2.log; then
        success_count=$((success_count + 1))
    fi
    if grep -q "Using prime field GF(4294967311)" node3.log; then
        success_count=$((success_count + 1))
    fi

    if [ $success_count -eq 3 ]; then
        success "All 3 nodes initialized with correct prime field"
    fi

    # Check for successful connections - should have 3 total
    connections_found=0

    # Node2 should connect to Node1 (1 connection)
    if grep -q "Successfully connected" node2.log; then
        success "Node2 connected to Node1"
        connections_found=$((connections_found + 1))
    else
        error "Node2 failed to connect to Node1"
    fi

    # Node3 should connect to both Node1 and Node2 (2 connections)
    if [ -f node3.log ]; then
        node3_connections=$(grep -c "Successfully connected" node3.log)
    else
        node3_connections=0
    fi
    if [ "$node3_connections" -eq 2 ]; then
        success "Node3 connected to both Node1 and Node2"
        connections_found=$((connections_found + 2))
    elif [ "$node3_connections" -eq 1 ]; then
        error "Node3 connected to only 1 node (expected 2)"
        connections_found=$((connections_found + 1))
    else
        error "Node3 failed to connect to any nodes"
    fi

    if [ $connections_found -ne 3 ]; then
        error "RLNC network test FAILED - not all connections established ($connections_found/3)"
        log "Node1 log:"
        cat node1.log 2>/dev/null || true
        log "Node2 log:"
        cat node2.log 2>/dev/null || true
        log "Node3 log:"
        cat node3.log 2>/dev/null || true
        exit 1
    fi

    success "RLNC network test PASSED - all 3 connections established successfully"
else
    error "RLNC network test FAILED - some nodes crashed"
    log "Node1 log:"
    cat node1.log 2>/dev/null || true
    log "Node2 log:"
    cat node2.log 2>/dev/null || true
    log "Node3 log:"
    cat node3.log 2>/dev/null || true
    exit 1
fi

# Test message publishing
log "Testing message publishing..."

# Send test message using a background process
{
    sleep 1
    echo "test-message-123"
    sleep 1
    echo "quit"
} | timeout 10s ./rlnc-network -l 8010 -c 127.0.0.1:8001 -id test-sender > test-sender.log 2>&1 &
TEST_PID=$!

# Wait for message to propagate
sleep 4

# Check if any nodes received the test message
received_count=0
if grep -q "Received message: test-message-123" node1.log; then
    received_count=$((received_count + 1))
fi
if grep -q "Received message: test-message-123" node2.log; then
    received_count=$((received_count + 1))
fi
if grep -q "Received message: test-message-123" node3.log; then
    received_count=$((received_count + 1))
fi

# Clean up test sender
kill $TEST_PID 2>/dev/null || true
wait $TEST_PID 2>/dev/null || true
rm -f test-sender.log

if [ $received_count -eq 3 ]; then
    success "Message publishing test PASSED - all 3 nodes received the message"
elif [ $received_count -gt 0 ]; then
    error "Message publishing test PARTIAL - only $received_count/3 nodes received the message"
else
    error "Message publishing test FAILED - no nodes received the message"
fi
