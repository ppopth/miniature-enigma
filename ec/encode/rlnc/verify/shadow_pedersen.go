package verify

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ethp2p/eth-ec-broadcast/ec/encode/rlnc"
	"github.com/ethp2p/eth-ec-broadcast/ec/field"
)

// ShadowPedersenVerifier is a lightweight verifier that simulates the timing of PedersenVerifier
// using sleep operations instead of actual cryptographic operations. This is useful for
// Shadow simulations where we want to model the performance impact without the computational cost.
type ShadowPedersenVerifier struct {
	chunkSize            int           // Expected network chunk size
	numChunks            int           // Expected number of chunks
	generateExtrasTiming time.Duration // Timing for GenerateExtras
	verifyTiming         time.Duration // Timing for Verify
}

// BenchmarkResult matches the JSON structure from benchmark_pedersen output
type BenchmarkResult struct {
	ChunkSize        int           `json:"chunk_size"`         // Network chunk size
	ElementsPerChunk int           `json:"elements_per_chunk"` // Number of field elements per chunk
	NumChunks        int           `json:"num_chunks"`
	Iterations       int           `json:"iterations"`
	GenerateExtras   time.Duration `json:"generate_extras_ns"`
	Verify           time.Duration `json:"verify_ns"`
}

// NewShadowPedersenVerifierFromFile creates a shadow verifier by loading timing from a benchmark file
func NewShadowPedersenVerifierFromFile(benchmarkFile string) (*ShadowPedersenVerifier, error) {
	data, err := os.ReadFile(benchmarkFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read benchmark file: %w", err)
	}

	var benchData BenchmarkResult
	if err := json.Unmarshal(data, &benchData); err != nil {
		return nil, fmt.Errorf("failed to parse benchmark file: %w", err)
	}

	return &ShadowPedersenVerifier{
		chunkSize:            benchData.ChunkSize,
		numChunks:            benchData.NumChunks,
		generateExtrasTiming: benchData.GenerateExtras,
		verifyTiming:         benchData.Verify,
	}, nil
}

// GenerateExtras simulates generating Pedersen commitments by sleeping
// The timing is based on the full GenerateExtras operation which includes:
// - Generating commitments for all chunks
// - Creating BLS signature
// - Serializing the extra data
func (sv *ShadowPedersenVerifier) GenerateExtras(chunks [][]byte) ([][]byte, error) {
	if len(chunks) == 0 {
		return [][]byte{}, nil
	}

	// Validate that the number of chunks matches
	if len(chunks) != sv.numChunks {
		return nil, fmt.Errorf("expected %d chunks, got %d", sv.numChunks, len(chunks))
	}

	// Validate chunk sizes
	for i, chunk := range chunks {
		if len(chunk) != sv.chunkSize {
			return nil, fmt.Errorf("chunk %d has size %d, expected %d", i, len(chunk), sv.chunkSize)
		}
	}

	// Sleep to simulate the computation
	time.Sleep(sv.generateExtrasTiming)

	// Return dummy extra data (same structure as real verifier)
	// Each chunk gets the same extra data in the real implementation
	// Extra data size: 4 (publisher ID) + 4 (count) + (numChunks × 32) + 96 (BLS sig)
	elementSize := 32 // Ristretto255 element size
	extraSize := 8 + (sv.numChunks * elementSize) + 96
	dummyExtra := make([]byte, extraSize)
	extras := make([][]byte, len(chunks))
	for i := range chunks {
		extras[i] = dummyExtra
	}

	return extras, nil
}

// CombineExtras simulates combining extra data (negligible time, no sleep needed)
func (sv *ShadowPedersenVerifier) CombineExtras(chunks []rlnc.Chunk, factors []field.Element) ([]byte, error) {
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks provided")
	}
	// Real implementation is very fast, so no sleep needed
	return chunks[0].Extra, nil
}

// Verify simulates verifying a chunk by sleeping
// The timing is based on the full Verify operation which includes:
// - Parsing extra data
// - Verifying BLS signature
// - Splitting chunk into elements
// - Computing commitment
// - Computing linear combination of stored commitments
// - Comparing commitments
func (sv *ShadowPedersenVerifier) Verify(chunk *rlnc.Chunk) bool {
	if chunk == nil {
		return false
	}

	// Validate that the number of coefficients matches expected chunks
	if len(chunk.Coeffs) != sv.numChunks {
		// Number of coefficients doesn't match - return false
		return false
	}

	// Simulate verification time
	time.Sleep(sv.verifyTiming)

	// Always return true for shadow simulation (assume valid)
	return true
}
