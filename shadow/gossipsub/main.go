package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"

	"github.com/ethp2p/eth-ec-broadcast/shadow/topology"

	logging "github.com/ipfs/go-log/v2"
)

func main() {
	var (
		nodeID       = flag.Int("node-id", 0, "Node ID for this simulation instance")
		nodeCount    = flag.Int("node-count", 10, "Total number of nodes in simulation")
		msgCount     = flag.Int("msg-count", 5, "Number of messages to publish")
		msgSize      = flag.Int("msg-size", 32, "Size of each message in bytes")
		topologyFile = flag.String("topology-file", "", "Path to topology JSON file (if not specified, uses linear topology)")
		logLevel     = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	)
	flag.Parse()

	// Setup logging
	log.SetPrefix(fmt.Sprintf("[gossipsub-node-%d] ", *nodeID))
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)

	// Set log level for all subsystems (including libp2p and pubsub)
	level, err := logging.LevelFromString(*logLevel)
	if err != nil {
		log.Printf("Invalid log level %q, using info", *logLevel)
		level = logging.LevelInfo
	}
	logging.SetAllLoggers(level)

	// Load topology
	var topo *topology.Topology

	if *topologyFile != "" {
		// Load from file
		topo, err = topology.LoadFromFile(*topologyFile)
		if err != nil {
			log.Fatalf("Failed to load topology from file: %v", err)
		}
		// Validate node count
		if topo.NodeCount != *nodeCount {
			log.Fatalf("Topology file specifies %d nodes but simulation has %d nodes",
				topo.NodeCount, *nodeCount)
		}
		log.Printf("Loaded topology from file: %s", *topologyFile)
	} else {
		// Default to linear topology
		topo = topology.GenerateLinear(*nodeCount)
		log.Printf("Using default linear topology")
	}

	log.Printf("Starting libp2p GossipSub simulation")
	log.Printf("Node ID: %d, Total nodes: %d, Messages: %d, Message size: %d bytes",
		*nodeID, *nodeCount, *msgCount, *msgSize)
	log.Printf("Topology: %s", topo.GetDescription())

	ctx := context.Background()

	// Generate a deterministic private key based on node ID
	// This ensures consistent peer IDs across simulation runs
	priv, _, err := crypto.GenerateEd25519Key(
		deterministicRandSource(*nodeID),
	)
	if err != nil {
		log.Fatalf("Failed to generate key: %v", err)
	}

	// Create libp2p host with deterministic port based on node ID
	hostPort := 8000 + *nodeID
	listenAddr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", hostPort)

	h, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings(listenAddr),
	)
	if err != nil {
		log.Fatalf("Failed to create libp2p host: %v", err)
	}
	defer h.Close()

	log.Printf("Host created with ID: %s, listening on port: %d", h.ID(), hostPort)

	// Create GossipSub instance with Ethereum consensus-specs parameters
	// Parameters from: https://github.com/ethereum/consensus-specs/blob/master/specs/phase0/p2p-interface.md
	// Start with default parameters and override specific values
	gossipsubParams := pubsub.DefaultGossipSubParams()
	gossipsubParams.D = 8                                      // topic stable mesh target count
	gossipsubParams.Dlo = 6                                    // topic stable mesh low watermark
	gossipsubParams.Dhi = 12                                   // topic stable mesh high watermark
	gossipsubParams.Dlazy = 6                                  // gossip target
	gossipsubParams.HeartbeatInterval = 700 * time.Millisecond // 0.7 seconds
	gossipsubParams.FanoutTTL = 60 * time.Second               // 60 seconds
	gossipsubParams.HistoryLength = 6                          // mcache_len: number of windows to retain full messages
	gossipsubParams.HistoryGossip = 3                          // mcache_gossip: number of windows to gossip about

	ps, err := pubsub.NewGossipSub(ctx, h,
		pubsub.WithGossipSubParams(gossipsubParams),
	)
	if err != nil {
		log.Fatalf("Failed to create gossipsub: %v", err)
	}

	// Join topic
	topicName := "gossipsub-sim"
	topic, err := ps.Join(topicName)
	if err != nil {
		log.Fatalf("Failed to join topic: %v", err)
	}

	// Subscribe to topic
	sub, err := topic.Subscribe()
	if err != nil {
		log.Fatalf("Failed to subscribe to topic: %v", err)
	}

	log.Printf("Successfully joined topic: %s", topicName)

	// Wait for all nodes to start listening (important in Shadow)
	log.Printf("Waiting for all nodes to initialize...")
	time.Sleep(3 * time.Second)

	// Helper function to connect to a peer by node ID
	connectToPeer := func(peerNodeID int) {
		hostname := fmt.Sprintf("node%d", peerNodeID)
		port := 8000 + peerNodeID

		// Resolve hostname to IP address
		addrs, err := net.LookupHost(hostname)
		if err != nil {
			log.Printf("Failed to resolve hostname %s: %v", hostname, err)
			return
		}
		if len(addrs) == 0 {
			log.Printf("No addresses found for hostname %s", hostname)
			return
		}

		// Create multiaddr
		maddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", addrs[0], port))
		if err != nil {
			log.Printf("Failed to create multiaddr: %v", err)
			return
		}

		// Parse peer ID (we need to know it in advance - use deterministic key generation)
		peerPriv, _, _ := crypto.GenerateEd25519Key(
			deterministicRandSource(peerNodeID),
		)
		peerID, _ := peer.IDFromPrivateKey(peerPriv)

		// Add peer address to peerstore
		peerInfo := peer.AddrInfo{
			ID:    peerID,
			Addrs: []multiaddr.Multiaddr{maddr},
		}

		err = h.Connect(ctx, peerInfo)
		if err != nil {
			log.Printf("Failed to connect to node %d: %v", peerNodeID, err)
		} else {
			log.Printf("Connected to node %d (peer ID: %s)", peerNodeID, peerID)
		}
	}

	// Connect to peers based on topology
	connections := topo.GetConnections(*nodeID)
	log.Printf("Node %d connections: %v", *nodeID, connections)

	for _, peerNodeID := range connections {
		// Only connect to higher-numbered peers to avoid duplicate connections
		if peerNodeID > *nodeID {
			connectToPeer(peerNodeID)
		}
	}

	// Wait for network setup and peer discovery (important in Shadow)
	log.Printf("Waiting for peer discovery and network stabilization...")
	time.Sleep(5 * time.Second)

	// Start message receiver goroutine
	receivedCount := 0
	go func() {
		for {
			msg, err := sub.Next(ctx)
			if err != nil {
				log.Printf("Error receiving message: %v", err)
				return
			}
			receivedCount++
			log.Printf("Received message %d: %s", receivedCount, string(msg.Data))
		}
	}()

	// Publish messages if this is node 0 (publisher)
	if *nodeID == 0 {
		log.Printf("Starting to publish %d messages", *msgCount)
		for i := 0; i < *msgCount; i++ {
			// Create message with specified size, fully filled
			msgContent := fmt.Sprintf("Message-%d-from-node-%d", i, *nodeID)
			msg := make([]byte, *msgSize)

			// Fill the entire message buffer
			for j := 0; j < *msgSize; j++ {
				if j < len(msgContent) {
					msg[j] = msgContent[j]
				} else {
					// Fill remaining bytes with a pattern to use full message size
					msg[j] = byte('A' + (j % 26))
				}
			}

			err := topic.Publish(ctx, msg)
			if err != nil {
				log.Printf("Failed to publish message %d: %v", i, err)
			} else {
				log.Printf("Published message %d: %s", i, msgContent)
			}

			// Wait between messages
			time.Sleep(1 * time.Second)
		}
	}

	// Keep the process running for Shadow
	log.Printf("Node %d running indefinitely for Shadow simulation", *nodeID)

	// Block forever to keep the process alive
	select {}
}

// deterministicRandSource creates a deterministic random source based on a seed
type deterministicRandSource int

func (d deterministicRandSource) Read(p []byte) (int, error) {
	// Simple deterministic "random" generation based on seed
	seed := int(d)
	for i := range p {
		seed = (seed*1103515245 + 12345) & 0x7fffffff
		p[i] = byte(seed >> 16)
	}
	return len(p), nil
}

// Ensure it implements io.Reader
var _ io.Reader = deterministicRandSource(0)
