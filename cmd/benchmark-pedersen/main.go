package main

import (
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ethp2p/eth-ec-broadcast/ec/encode/rlnc"
	"github.com/ethp2p/eth-ec-broadcast/ec/encode/rlnc/verify"
	"github.com/ethp2p/eth-ec-broadcast/ec/field"
	"github.com/ethp2p/eth-ec-broadcast/ec/group"
	"github.com/herumi/bls-eth-go-binary/bls"
)

// BenchmarkResult stores timing data for PedersenVerifier operations
type BenchmarkResult struct {
	ChunkSize        int           `json:"chunk_size"`         // Network chunk size
	ElementsPerChunk int           `json:"elements_per_chunk"` // Number of field elements per chunk
	NumChunks        int           `json:"num_chunks"`
	Iterations       int           `json:"iterations"`
	GenerateExtras   time.Duration `json:"generate_extras_ns"` // Average time for GenerateExtras
	Verify           time.Duration `json:"verify_ns"`          // Average time for Verify
}

func main() {
	// Parse command-line flags
	messageChunkSize := flag.Int("chunk-size", 1024, "Message chunk size in bytes (not network chunk size)")
	numChunks := flag.Int("num-chunks", 8, "Number of chunks to benchmark")
	iterations := flag.Int("iterations", 100, "Number of iterations per benchmark")
	outputFile := flag.String("output", "pedersen_benchmark.json", "Output file for benchmark results")
	flag.Parse()

	messageBytesPerElement := 32
	networkBytesPerElement := 33

	// Validate that message chunk size is compatible with bytes per element
	if *messageChunkSize%messageBytesPerElement != 0 {
		fmt.Fprintf(os.Stderr, "Error: Message chunk size %d must be a multiple of %d bytes per element\n",
			*messageChunkSize, messageBytesPerElement)
		os.Exit(1)
	}

	elementsPerChunk := *messageChunkSize / messageBytesPerElement

	// Calculate bits per element for message and network
	messageBitsPerElement := 8 * messageBytesPerElement
	networkBitsPerElement := 8 * networkBytesPerElement
	networkChunkSize := elementsPerChunk * networkBytesPerElement

	fmt.Printf("Benchmarking PedersenVerifier with:\n")
	fmt.Printf("  Message chunk size: %d bytes\n", *messageChunkSize)
	fmt.Printf("  Network chunk size: %d bytes\n", networkChunkSize)
	fmt.Printf("  Elements per chunk: %d\n", elementsPerChunk)
	fmt.Printf("  Number of chunks: %d\n", *numChunks)
	fmt.Printf("  Iterations: %d\n", *iterations)
	fmt.Println()

	// Initialize BLS
	if err := bls.Init(bls.BLS12_381); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize BLS: %v\n", err)
		os.Exit(1)
	}
	if err := bls.SetETHmode(bls.EthModeDraft07); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set ETH mode: %v\n", err)
		os.Exit(1)
	}

	// Create field for converting message chunks to network chunks
	f := group.NewScalarField(verify.Ristretto255Group)

	// Create BLS keys
	sk := &bls.SecretKey{}
	sk.SetByCSPRNG()
	pk := sk.GetPublicKey()

	// Create PedersenVerifier with network chunk size
	pv, err := verify.NewPedersenVerifier(networkChunkSize, elementsPerChunk, &verify.PedersenConfig{
		BLSSecretKey: sk,
		PublicKeyCallback: func(publisherID int) *bls.PublicKey {
			return pk
		},
		PublisherID: 0,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create PedersenVerifier: %v\n", err)
		os.Exit(1)
	}

	// Create test chunks: generate message chunks and convert to network chunks
	chunks := make([][]byte, *numChunks)
	for i := 0; i < *numChunks; i++ {
		// Generate random message chunk
		messageChunk := make([]byte, *messageChunkSize)
		rand.Read(messageChunk)

		// Convert to field elements and re-encode for network transmission
		// This matches what rlnc.go does in GenerateThenAddChunks
		fieldElements := field.SplitBitsToFieldElements(messageChunk, messageBitsPerElement, f)
		networkChunk := field.FieldElementsToBytes(fieldElements, networkBitsPerElement)

		chunks[i] = networkChunk
	}

	result := BenchmarkResult{
		ChunkSize:        networkChunkSize,
		ElementsPerChunk: elementsPerChunk,
		NumChunks:        *numChunks,
		Iterations:       *iterations,
	}

	// Benchmark GenerateExtras
	fmt.Print("Benchmarking GenerateExtras... ")
	start := time.Now()
	for i := 0; i < *iterations; i++ {
		_, err := pv.GenerateExtras(chunks)
		if err != nil {
			fmt.Fprintf(os.Stderr, "GenerateExtras failed: %v\n", err)
			os.Exit(1)
		}
	}
	result.GenerateExtras = time.Since(start) / time.Duration(*iterations)
	fmt.Printf("%v\n", result.GenerateExtras)

	// Benchmark Verify by creating a chunk manually
	fmt.Print("Benchmarking Verify... ")

	// Generate extras to get valid extra data
	extras, err := pv.GenerateExtras(chunks)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate extras: %v\n", err)
		os.Exit(1)
	}

	// Create a test chunk with the extra data and coefficients
	coeffs := make([]field.Element, *numChunks)
	for i := 0; i < *numChunks; i++ {
		// Create random coefficients
		coeffBytes := make([]byte, 2)
		rand.Read(coeffBytes)
		coeffs[i] = f.FromBytes(coeffBytes)
	}

	testChunk := &rlnc.Chunk{
		MessageID: "bench-message",
		ChunkData: chunks[0], // Use first chunk data
		Coeffs:    coeffs,
		Extra:     extras[0],
	}

	// Benchmark Verify
	start = time.Now()
	for i := 0; i < *iterations; i++ {
		_ = pv.Verify(testChunk)
	}
	result.Verify = time.Since(start) / time.Duration(*iterations)
	fmt.Printf("%v\n", result.Verify)

	// Write result to file
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal results: %v\n", err)
		os.Exit(1)
	}

	err = os.WriteFile(*outputFile, data, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write results to file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nBenchmark results written to: %s\n", *outputFile)
}
