package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/ethp2p/eth-ec-broadcast/ec"
	"github.com/ethp2p/eth-ec-broadcast/ec/encode/rs"
	"github.com/ethp2p/eth-ec-broadcast/ec/field"
	"github.com/ethp2p/eth-ec-broadcast/host"
	"github.com/ethp2p/eth-ec-broadcast/pubsub"
)

// Hard-coded Reed-Solomon parameters for optimal performance
const (
	redundancyRatio = 1.0 // 100% redundancy - good balance between efficiency and reliability
	chunkSize       = 8   // 8 bytes per chunk - efficient for small to medium messages
)

var (
	listenFlag  = flag.Uint("l", 0, "the listening port")
	connectFlag = flag.String("c", "", "comma-separated list of remote addresses to connect to")
	nodeIDFlag  = flag.String("id", "", "node identifier for logging")
	topicFlag   = flag.String("topic", "rs-demo", "topic name to join")
	verboseFlag = flag.Bool("v", false, "verbose logging")
)

func main() {
	flag.Parse()

	if *nodeIDFlag == "" {
		*nodeIDFlag = fmt.Sprintf("node-%d", *listenFlag)
	}

	log.SetPrefix(fmt.Sprintf("[%s] ", *nodeIDFlag))

	// Create host with specified port
	var opts []host.HostOption
	if *listenFlag != 0 {
		opts = append(opts, host.WithAddrPort(
			netip.AddrPortFrom(netip.IPv4Unspecified(), uint16(*listenFlag)),
		))
	}

	h, err := host.NewHost(opts...)
	if err != nil {
		log.Fatalf("Failed to create host: %v", err)
	}
	defer h.Close()

	log.Printf("Host started, listening on %s", h.LocalAddr())

	// Create PubSub instance
	ps, err := pubsub.NewPubSub(h)
	if err != nil {
		log.Fatalf("Failed to create pubsub: %v", err)
	}
	defer ps.Close()

	// Hard-coded field configuration
	// Using GF(2^8) - the standard field for Reed-Solomon implementations
	// This provides optimal performance for byte-oriented operations
	irreducible := big.NewInt(0x11B) // x^8 + x^4 + x^3 + x + 1 (standard AES polynomial)
	f := field.NewBinaryField(8, irreducible)
	primitiveElement := f.FromBytes([]byte{0x03}) // 3 is a primitive element in GF(2^8)
	log.Printf("Using binary field GF(2^8) - standard for Reed-Solomon")

	// Configure Reed-Solomon encoder with optimal parameters
	rsConfig := &rs.RsEncoderConfig{
		ParityRatio:      redundancyRatio, // 100% redundancy
		MessageChunkSize: chunkSize,       // 8 bytes per chunk
		NetworkChunkSize: chunkSize,       // 8 bytes per chunk
		ElementsPerChunk: chunkSize,       // Each element is 1 byte
		Field:            f,
		PrimitiveElement: primitiveElement,
	}

	encoder, err := rs.NewRsEncoder(rsConfig)
	if err != nil {
		log.Fatalf("Failed to create RS encoder: %v", err)
	}

	// Create EC router with Reed-Solomon
	ecRouter, err := ec.NewEcRouter(encoder,
		ec.WithEcParams(ec.EcParams{
			PublishMultiplier: 4, // Send 4x redundancy when publishing
			ForwardMultiplier: 8, // Forward 8x when relaying
		}),
	)
	if err != nil {
		log.Fatalf("Failed to create EC router: %v", err)
	}

	// Join topic with RS router
	topic, err := ps.Join(*topicFlag, ecRouter)
	if err != nil {
		log.Fatalf("Failed to join topic: %v", err)
	}
	defer topic.Close()

	// Subscribe to receive messages
	sub, err := topic.Subscribe()
	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	log.Printf("Joined topic '%s' with Reed-Solomon erasure coding", *topicFlag)
	log.Printf("RS Config: ChunkSize=%d bytes, Redundancy=%.1f%%, Field=GF(2^8)",
		chunkSize, redundancyRatio*100)

	// Connect to remote peers if specified
	if *connectFlag != "" {
		addresses := strings.Split(*connectFlag, ",")
		for _, addrStr := range addresses {
			addrStr = strings.TrimSpace(addrStr)
			if addrStr == "" {
				continue
			}

			addr, err := net.ResolveUDPAddr("udp", addrStr)
			if err != nil {
				log.Printf("Failed to resolve address %s: %v", addrStr, err)
				continue
			}

			log.Printf("Connecting to %s...", addr)
			if err := h.Connect(context.Background(), addr); err != nil {
				log.Printf("Failed to connect to %s: %v", addr, err)
			} else {
				log.Printf("Successfully connected to %s", addr)
			}
		}
	}

	// Start message receiver goroutine
	go func() {
		for {
			msg, err := sub.Next(context.Background())
			if err != nil {
				log.Printf("Subscription error: %v", err)
				return
			}

			log.Printf("✓ Received message: %s", string(msg))

			// Display reconstruction statistics if verbose
			if *verboseFlag {
				messageIDs := encoder.GetMessageIDs()
				for _, msgID := range messageIDs {
					chunkCount := encoder.GetChunkCount(msgID)
					minChunks := encoder.GetMinChunksForReconstruction(msgID)
					log.Printf("  Message ID %x: %d/%d chunks received", msgID[:8], chunkCount, minChunks)
				}
			}
		}
	}()

	// Wait for connections to establish
	time.Sleep(500 * time.Millisecond)

	// Interactive message sending
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Printf("\n=== Reed-Solomon Network Demo ===\n")
	fmt.Printf("Field: Binary GF(2^8)\n")
	fmt.Printf("Topic: %s\n", *topicFlag)
	fmt.Printf("Node: %s\n", *nodeIDFlag)
	fmt.Printf("Redundancy: %.1f%%\n", redundancyRatio*100)
	fmt.Printf("\nType messages to broadcast (press Enter to send, 'quit' to exit):\n")

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		message := strings.TrimSpace(scanner.Text())
		if message == "" {
			continue
		}
		if message == "quit" || message == "exit" {
			break
		}

		// Special commands
		if strings.HasPrefix(message, "/") {
			handleCommand(message, encoder, ecRouter)
			continue
		}

		// Publish message
		log.Printf("Publishing message: %s", message)
		start := time.Now()
		err := topic.Publish([]byte(message))
		if err != nil {
			log.Printf("Failed to publish message: %v", err)
		} else {
			duration := time.Since(start)
			log.Printf("✓ Message published successfully (took %v)", duration)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Scanner error: %v", err)
	}

	log.Println("Shutting down...")
}

// handleCommand processes special commands
func handleCommand(cmd string, encoder *rs.RsEncoder, router *ec.EcRouter) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "/help":
		fmt.Println("Available commands:")
		fmt.Println("  /help       - Show this help")
		fmt.Println("  /stats      - Show encoding statistics")
		fmt.Println("  /messages   - List received message IDs")
		fmt.Println("  /chunks     - Show chunk counts and reconstruction status")

	case "/stats":
		fmt.Printf("Reed-Solomon Encoder Statistics:\n")
		messageIDs := encoder.GetMessageIDs()
		fmt.Printf("  Total messages: %d\n", len(messageIDs))

		totalChunks := 0
		reconstructible := 0
		for _, msgID := range messageIDs {
			chunkCount := encoder.GetChunkCount(msgID)
			totalChunks += chunkCount
			minChunks := encoder.GetMinChunksForReconstruction(msgID)
			if chunkCount >= minChunks {
				reconstructible++
			}
		}
		fmt.Printf("  Total chunks: %d\n", totalChunks)
		fmt.Printf("  Reconstructible messages: %d/%d\n", reconstructible, len(messageIDs))

	case "/messages":
		messageIDs := encoder.GetMessageIDs()
		fmt.Printf("Received message IDs (%d total):\n", len(messageIDs))
		for i, msgID := range messageIDs {
			fmt.Printf("  %d: %x\n", i+1, msgID[:8])
		}

	case "/chunks":
		messageIDs := encoder.GetMessageIDs()
		fmt.Printf("Chunk status for all messages:\n")
		for _, msgID := range messageIDs {
			chunkCount := encoder.GetChunkCount(msgID)
			minChunks := encoder.GetMinChunksForReconstruction(msgID)

			// Try to reconstruct to see if we have enough chunks
			_, err := encoder.ReconstructMessage(msgID)
			status := "✓ Complete"
			if err != nil {
				if chunkCount >= minChunks {
					status = "⚠ Error reconstructing"
				} else {
					status = fmt.Sprintf("⚠ Need %d more chunks", minChunks-chunkCount)
				}
			}
			fmt.Printf("  %x: %d/%d chunks %s\n", msgID[:8], chunkCount, minChunks, status)
		}

	default:
		fmt.Printf("Unknown command: %s. Type /help for available commands.\n", parts[0])
	}
}
