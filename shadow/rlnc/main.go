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
	"github.com/ethp2p/eth-ec-broadcast/ec/encode/rlnc"
	"github.com/ethp2p/eth-ec-broadcast/ec/field"
	"github.com/ethp2p/eth-ec-broadcast/host"
	"github.com/ethp2p/eth-ec-broadcast/pubsub"
)

func main() {
	var (
		nodeID    = flag.Int("node-id", 0, "Node ID for this simulation instance")
		nodeCount = flag.Int("node-count", 10, "Total number of nodes in simulation")
		msgCount  = flag.Int("msg-count", 5, "Number of messages to publish")
		msgSize   = flag.Int("msg-size", 32, "Size of each message in bytes")
	)
	flag.Parse()

	// Setup logging
	log.SetPrefix(fmt.Sprintf("[rlnc-node-%d] ", *nodeID))
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	log.Printf("Starting eth-ec-broadcast RLNC simulation")
	log.Printf("Node ID: %d, Total nodes: %d, Messages: %d, Message size: %d bytes",
		*nodeID, *nodeCount, *msgCount, *msgSize)

	// Create host with deterministic port based on node ID
	// Enable Shadow compatibility mode
	hostPort := uint16(8000 + *nodeID)
	h, err := host.NewHost(
		host.WithAddrPort(netip.AddrPortFrom(netip.IPv4Unspecified(), hostPort)),
		host.WithShadowMode(),
	)
	if err != nil {
		log.Fatalf("Failed to create host: %v", err)
	}
	defer h.Close()

	log.Printf("Host created with ID: %s, listening on port: %d (Shadow mode)", h.ID(), hostPort)

	// Create PubSub instance
	ps, err := pubsub.NewPubSub(h)
	if err != nil {
		log.Fatalf("Failed to create pubsub: %v", err)
	}
	defer ps.Close()

	// Create RLNC encoder with messageChunkSize = 8 bytes
	// For 8 bytes (64 bits) with ElementsPerChunk = 1:
	// messageBitsPerElement = 64/1 = 64, so need BitsPerDataElement() ≥ 64
	// This means p.BitLen() ≥ 65, so use a 65-bit prime
	prime65bit := new(big.Int)
	prime65bit.SetString("36893488147419103183", 10) // 2^65 - 49, a 65-bit prime
	f := field.NewPrimeField(prime65bit)

	messageChunkSize := 8
	elementsPerChunk := 1 // One 65-bit field element per chunk
	networkChunkSize := 9 // Need at least ⌈65/8⌉ = 9 bytes for network serialization

	// Validate that message size is compatible with chunk size
	if *msgSize%messageChunkSize != 0 {
		log.Fatalf("Message size %d must be a multiple of message chunk size %d",
			*msgSize, messageChunkSize)
	}

	rlncConfig := &rlnc.RlncEncoderConfig{
		MessageChunkSize:   messageChunkSize, // 8 bytes
		NetworkChunkSize:   networkChunkSize, // 9 bytes
		ElementsPerChunk:   elementsPerChunk, // 1 element
		MaxCoefficientBits: 32,
		Field:              f,
	}
	encoder, err := rlnc.NewRlncEncoder(rlncConfig)
	if err != nil {
		log.Fatalf("Failed to create RLNC encoder: %v", err)
	}

	// Create EC router with RLNC
	router, err := ec.NewEcRouter(encoder, ec.WithEcParams(ec.EcParams{
		PublishMultiplier: 2, // 2x redundancy when publishing
		ForwardMultiplier: 2, // 2x when forwarding
	}))
	if err != nil {
		log.Fatalf("Failed to create EC router: %v", err)
	}

	// Join topic
	topicName := "rlnc-sim"
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

	log.Printf("Successfully joined topic: %s with RLNC encoding", topicName)

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

	// Connect to next node only (linear topology: 0->1->2->3...)
	if *nodeID < *nodeCount-1 {
		connectToPeer(*nodeID + 1) // Connect to next node
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
			log.Printf("Received message %d (reconstructed from RLNC chunks): %s", receivedCount, string(msg))
		}
	}()

	// Publish messages if this is node 0 (publisher)
	if *nodeID == 0 {
		log.Printf("Starting to publish %d messages with RLNC encoding", *msgCount)
		for i := 0; i < *msgCount; i++ {
			// Create message with specified size, fully filled
			msgContent := fmt.Sprintf("RLNCMsg-%d-from-node-%d", i, *nodeID)
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
				log.Printf("Published message %d with RLNC encoding: %s", i, msgContent)
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
