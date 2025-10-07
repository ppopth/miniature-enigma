package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/netip"
	"time"

	"github.com/ethp2p/eth-ec-broadcast/ec"
	"github.com/ethp2p/eth-ec-broadcast/ec/encode/rs"
	"github.com/ethp2p/eth-ec-broadcast/ec/field"
	"github.com/ethp2p/eth-ec-broadcast/host"
	"github.com/ethp2p/eth-ec-broadcast/pubsub"
	"github.com/ethp2p/eth-ec-broadcast/shadow/topology"

	logging "github.com/ipfs/go-log/v2"
)

func main() {
	var (
		nodeID       = flag.Int("node-id", 0, "Node ID for this simulation instance")
		nodeCount    = flag.Int("node-count", 10, "Total number of nodes in simulation")
		msgCount     = flag.Int("msg-count", 5, "Number of messages to publish")
		msgSize      = flag.Int("msg-size", 32, "Size of each message in bytes")
		numChunks    = flag.Int("num-chunks", 4, "Number of chunks to divide each message into")
		multiplier   = flag.Int("multiplier", 4, "Multiplier for publish, forward, and min emit count")
		topologyFile = flag.String("topology-file", "", "Path to topology JSON file (if not specified, uses linear topology)")
		useStreams   = flag.Bool("use-streams", false, "Use QUIC streams instead of datagrams")
		logLevel     = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	)
	flag.Parse()

	// Setup logging
	log.SetPrefix(fmt.Sprintf("[rs-node-%d] ", *nodeID))
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)

	// Set log level for all subsystems
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

	log.Printf("Starting eth-ec-broadcast Reed-Solomon simulation")
	log.Printf("Node ID: %d, Total nodes: %d, Messages: %d, Message size: %d bytes, Chunks: %d",
		*nodeID, *nodeCount, *msgCount, *msgSize, *numChunks)
	log.Printf("Topology: %s", topo.GetDescription())

	// Create host with deterministic port based on node ID
	// Enable Shadow compatibility mode
	hostPort := uint16(8000 + *nodeID)

	// Configure transport mode
	transportMode := host.TransportDatagram
	transportName := "datagram"
	if *useStreams {
		transportMode = host.TransportStream
		transportName = "stream"
	}

	h, err := host.NewHost(
		host.WithAddrPort(netip.AddrPortFrom(netip.IPv4Unspecified(), hostPort)),
		host.WithShadowMode(),
		host.WithTransportMode(transportMode),
	)
	if err != nil {
		log.Fatalf("Failed to create host: %v", err)
	}
	defer h.Close()

	log.Printf("Host created with ID: %s, listening on port: %d (Shadow mode, transport: %s)", h.ID(), hostPort, transportName)

	// Create PubSub instance
	ps, err := pubsub.NewPubSub(h)
	if err != nil {
		log.Fatalf("Failed to create pubsub: %v", err)
	}
	defer ps.Close()

	// Calculate chunk size based on message size and number of chunks
	if *msgSize%*numChunks != 0 {
		log.Fatalf("Message size %d must be divisible by number of chunks %d",
			*msgSize, *numChunks)
	}

	messageChunkSize := *msgSize / *numChunks
	log.Printf("Calculated chunk size: %d bytes (message size %d / %d chunks)",
		messageChunkSize, *msgSize, *numChunks)

	// Create Reed-Solomon encoder with binary field GF(2^8)
	irreducible := big.NewInt(0x11B) // x^8 + x^4 + x^3 + x + 1
	f := field.NewBinaryField(8, irreducible)

	// For GF(2^8): Each field element can hold 1 byte
	bytesPerElement := 1

	// Validate that chunk size is compatible with bytes per element
	if messageChunkSize%bytesPerElement != 0 {
		log.Fatalf("Chunk size %d must be a multiple of %d bytes per element",
			messageChunkSize, bytesPerElement)
	}

	elementsPerChunk := messageChunkSize / bytesPerElement
	networkChunkSize := messageChunkSize // Same size since each element fits in 1 byte

	log.Printf("Chunk configuration: %d bytes per chunk, %d elements per chunk, %d bytes network chunk size",
		messageChunkSize, elementsPerChunk, networkChunkSize)

	// Divide multiplier by 2 since ParityRatio is 100% (already 2x redundancy)
	effectiveMultiplier := *multiplier / 2
	if effectiveMultiplier < 1 {
		effectiveMultiplier = 1
	}

	rsConfig := &rs.RsEncoderConfig{
		ParityRatio:      1.0, // 100% redundancy
		MessageChunkSize: messageChunkSize,
		NetworkChunkSize: networkChunkSize,
		ElementsPerChunk: elementsPerChunk,
		Field:            f,
		PrimitiveElement: f.FromBytes([]byte{0x03}), // 0x03 is primitive in GF(2^8)
		MinEmitCount:     effectiveMultiplier,
	}
	encoder, err := rs.NewRsEncoder(rsConfig)
	if err != nil {
		log.Fatalf("Failed to create Reed-Solomon encoder: %v", err)
	}

	// Create EC router with Reed-Solomon
	router, err := ec.NewEcRouter(encoder, ec.WithEcParams(ec.EcParams{
		PublishMultiplier: effectiveMultiplier,
		ForwardMultiplier: effectiveMultiplier,
	}))
	if err != nil {
		log.Fatalf("Failed to create EC router: %v", err)
	}

	// Join topic
	topicName := "rs-sim"
	topic, err := ps.Join(topicName, router)
	if err != nil {
		log.Fatalf("Failed to join topic: %v", err)
	}
	defer topic.Close()

	// Subscribe to topic
	sub, err := topic.Subscribe()
	if err != nil {
		log.Fatalf("Failed to subscribe to topic: %v", err)
	}

	log.Printf("Successfully joined topic: %s with Reed-Solomon encoding", topicName)

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

		// Use the first resolved address
		ip, err := netip.ParseAddr(addrs[0])
		if err != nil {
			log.Printf("Failed to parse IP address %s: %v", addrs[0], err)
			return
		}

		peerAddr := &net.UDPAddr{
			IP:   ip.AsSlice(),
			Port: port,
		}

		err = h.Connect(context.Background(), peerAddr)
		if err != nil {
			log.Printf("Failed to connect to node %d (%s): %v", peerNodeID, peerAddr, err)
		} else {
			log.Printf("Connected to node %d at %s", peerNodeID, peerAddr)
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
			msg, err := sub.Next(context.Background())
			if err != nil {
				log.Printf("Error receiving message: %v", err)
				return
			}
			receivedCount++
			log.Printf("Received message %d (reconstructed from RS chunks): %s", receivedCount, string(msg))
		}
	}()

	// Publish messages if this is node 0 (publisher)
	if *nodeID == 0 {
		log.Printf("Starting to publish %d messages with Reed-Solomon encoding", *msgCount)
		for i := 0; i < *msgCount; i++ {
			// Create message with specified size, fully filled
			msgContent := fmt.Sprintf("RSMsg-%d-from-node-%d", i, *nodeID)
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

			err := topic.Publish(msg)
			if err != nil {
				log.Printf("Failed to publish message %d: %v", i, err)
			} else {
				log.Printf("Published message %d with Reed-Solomon encoding: %s", i, msgContent)
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
