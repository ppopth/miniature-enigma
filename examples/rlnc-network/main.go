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
	"github.com/ethp2p/eth-ec-broadcast/ec/encode/rlnc"
	"github.com/ethp2p/eth-ec-broadcast/ec/field"
	"github.com/ethp2p/eth-ec-broadcast/host"
	"github.com/ethp2p/eth-ec-broadcast/pubsub"
)

var (
	listenFlag  = flag.Uint("l", 0, "the listening port")
	connectFlag = flag.String("c", "", "comma-separated list of remote addresses to connect to (e.g., 127.0.0.1:8001,127.0.0.1:8002)")
	nodeIDFlag  = flag.String("id", "", "node identifier for logging")
	topicFlag   = flag.String("topic", "rlnc-demo", "topic name to join")
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
	// Using the first prime larger than 2^32 for good performance and security
	// Prime: 2^32 + 15 = 4,294,967,311 (first prime larger than 2^32 = 4,294,967,296)
	f := field.NewPrimeField(big.NewInt(4_294_967_311))
	log.Printf("Using prime field GF(4294967311) - first prime larger than 2^32")

	// Configure RLNC encoder with optimal parameters
	rlncConfig := &rlnc.RlncEncoderConfig{
		MessageChunkSize:   8,  // 8 bytes per message chunk
		NetworkChunkSize:   9,  // 9 bytes per network chunk
		ElementsPerChunk:   2,  // 2 field elements per chunk
		MaxCoefficientBits: 32, // 32-bit coefficients
		Field:              f,
	}

	encoder, err := rlnc.NewRlncEncoder(rlncConfig)
	if err != nil {
		log.Fatalf("Failed to create RLNC encoder: %v", err)
	}

	// Create EC router with RLNC
	ecRouter, err := ec.NewEcRouter(encoder,
		ec.WithEcParams(ec.EcParams{
			PublishMultiplier: 4, // Send 4x redundancy when publishing
			ForwardMultiplier: 8, // Forward 8x when relaying
		}),
	)
	if err != nil {
		log.Fatalf("Failed to create EC router: %v", err)
	}

	// Join topic with RLNC router
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

	log.Printf("Joined topic '%s' with RLNC erasure coding", *topicFlag)
	log.Printf("RLNC Config: ChunkSize=%d", rlncConfig.MessageChunkSize)

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
					log.Printf("  Message ID %x: %d chunks received", msgID[:8], chunkCount)
				}
			}
		}
	}()

	// Wait for connections to establish
	time.Sleep(500 * time.Millisecond)

	// Interactive message sending
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Printf("\n=== RLNC Network Demo ===\n")
	fmt.Printf("Field: Prime GF(4294967311) - first prime > 2^32\n")
	fmt.Printf("Topic: %s\n", *topicFlag)
	fmt.Printf("Node: %s\n", *nodeIDFlag)
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
func handleCommand(cmd string, encoder *rlnc.RlncEncoder, router *ec.EcRouter) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "/help":
		fmt.Println("Available commands:")
		fmt.Println("  /help     - Show this help")
		fmt.Println("  /stats    - Show encoding statistics")
		fmt.Println("  /messages - List received message IDs")
		fmt.Println("  /chunks   - Show chunk counts for all messages")

	case "/stats":
		fmt.Printf("Encoder Statistics:\n")
		messageIDs := encoder.GetMessageIDs()
		fmt.Printf("  Total messages: %d\n", len(messageIDs))

		totalChunks := 0
		for _, msgID := range messageIDs {
			totalChunks += encoder.GetChunkCount(msgID)
		}
		fmt.Printf("  Total chunks: %d\n", totalChunks)

	case "/messages":
		messageIDs := encoder.GetMessageIDs()
		fmt.Printf("Received message IDs (%d total):\n", len(messageIDs))
		for i, msgID := range messageIDs {
			fmt.Printf("  %d: %x\n", i+1, msgID[:8])
		}

	case "/chunks":
		messageIDs := encoder.GetMessageIDs()
		fmt.Printf("Chunk counts for all messages:\n")
		for _, msgID := range messageIDs {
			chunkCount := encoder.GetChunkCount(msgID)
			// Try to reconstruct to see if we have enough chunks
			_, err := encoder.ReconstructMessage(msgID)
			status := "✓ Complete"
			if err != nil {
				status = "⚠ Incomplete"
			}
			fmt.Printf("  %x: %d chunks %s\n", msgID[:8], chunkCount, status)
		}

	default:
		fmt.Printf("Unknown command: %s. Type /help for available commands.\n", parts[0])
	}
}
