package rlnc

import (
	"math/big"
	"slices"
	"testing"

	"github.com/ethp2p/eth-ec-broadcast/ec/encode"
	"github.com/ethp2p/eth-ec-broadcast/ec/field"
)

func TestRlncEncoderBasic(t *testing.T) {
	// Create a prime field for testing
	f := field.NewPrimeField(big.NewInt(4_294_967_311))

	config := &RlncEncoderConfig{
		MessageChunkSize:   8,
		NetworkChunkSize:   9,
		ElementsPerChunk:   2,
		MaxCoefficientBits: 16,
		Field:              f,
	}

	encoder, err := NewRlncEncoder(config)
	if err != nil {
		t.Fatal(err)
	}

	// Test message generation
	msg := []byte("hello123")
	messageID := "test-message-1"

	numChunks, err := encoder.GenerateThenAddChunks(messageID, msg)
	if err != nil {
		t.Fatal(err)
	}

	// Should have 1 chunk for 8-byte message with 8-byte chunks
	if numChunks != 1 {
		t.Fatalf("expected 1 chunk, got %d", numChunks)
	}

	// Test chunk data access
	chunks := encoder.GetChunks(messageID)
	chunk := chunks[0]
	if len(chunk.Data()) != 9 { // Network chunk size
		t.Fatalf("expected chunk data length 9, got %d", len(chunk.Data()))
	}

	// Test reconstruction
	reconstructed, err := encoder.ReconstructMessage(messageID)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(reconstructed, msg) {
		t.Fatalf("reconstructed message doesn't match original: got %v, want %v", reconstructed, msg)
	}
}

func TestRlncEncoderMultipleChunks(t *testing.T) {
	f := field.NewPrimeField(big.NewInt(4_294_967_311))

	config := &RlncEncoderConfig{
		MessageChunkSize:   4,
		NetworkChunkSize:   16,
		ElementsPerChunk:   2,
		MaxCoefficientBits: 16,
		Field:              f,
	}

	encoder, err := NewRlncEncoder(config)
	if err != nil {
		t.Fatal(err)
	}

	// Test message that will create multiple chunks
	msg := []byte("abcdefghijklmnop") // 16 bytes -> 4 chunks of 4 bytes each
	messageID := "test-message-multi"

	numChunks, err := encoder.GenerateThenAddChunks(messageID, msg)
	if err != nil {
		t.Fatal(err)
	}

	// Should have 4 chunks
	if numChunks != 4 {
		t.Fatalf("expected 4 chunks, got %d", numChunks)
	}

	// Test that each chunk has proper coefficient length
	chunks := encoder.GetChunks(messageID)
	for i, chunk := range chunks {
		if len(chunk.Coeffs) != 4 {
			t.Fatalf("chunk %d has %d coefficients, expected 4", i, len(chunk.Coeffs))
		}
	}

	// Test reconstruction
	reconstructed, err := encoder.ReconstructMessage(messageID)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(reconstructed, msg) {
		t.Fatalf("reconstructed message doesn't match original: got %v, want %v", reconstructed, msg)
	}
}

func TestRlncEncoderVerifyThenAddChunk(t *testing.T) {
	f := field.NewPrimeField(big.NewInt(4_294_967_311))

	config := &RlncEncoderConfig{
		MessageChunkSize:   4,
		NetworkChunkSize:   16,
		ElementsPerChunk:   2,
		MaxCoefficientBits: 16,
		Field:              f,
	}

	encoder, err := NewRlncEncoder(config)
	if err != nil {
		t.Fatal(err)
	}

	msg := []byte("testdata")
	messageID := "test-verify"

	// Generate chunks
	_, err = encoder.GenerateThenAddChunks(messageID, msg)
	if err != nil {
		t.Fatal(err)
	}
	chunks := encoder.GetChunks(messageID)

	// Create a second encoder to simulate receiving chunks
	encoder2, err := NewRlncEncoder(config)
	if err != nil {
		t.Fatal(err)
	}

	// Add chunks to second encoder
	for _, chunk := range chunks {
		if !encoder2.VerifyThenAddChunk(chunk) {
			t.Fatal("failed to add valid chunk")
		}
	}

	// Test reconstruction in second encoder
	reconstructed, err := encoder2.ReconstructMessage(messageID)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(reconstructed, msg) {
		t.Fatalf("reconstructed message doesn't match original: got %v, want %v", reconstructed, msg)
	}
}

func TestRlncEncoderEmitChunk(t *testing.T) {
	f := field.NewPrimeField(big.NewInt(4_294_967_311))

	config := &RlncEncoderConfig{
		MessageChunkSize:   4,
		NetworkChunkSize:   16,
		ElementsPerChunk:   2,
		MaxCoefficientBits: 16,
		Field:              f,
	}

	encoder, err := NewRlncEncoder(config)
	if err != nil {
		t.Fatal(err)
	}

	msg := []byte("testdata")
	messageID := "test-verify"

	// Generate chunks
	numChunks, err := encoder.GenerateThenAddChunks(messageID, msg)
	if err != nil {
		t.Fatal(err)
	}

	var combinedChunks []encode.Chunk
	for i := 0; i < numChunks; i++ {
		combinedChunk, err := encoder.EmitChunk(messageID)
		if err != nil {
			t.Fatal(err)
		}
		combinedChunks = append(combinedChunks, combinedChunk)
	}

	// Create a second encoder to simulate receiving chunks
	encoder2, err := NewRlncEncoder(config)
	if err != nil {
		t.Fatal(err)
	}

	// Add chunks to second encoder
	for _, chunk := range combinedChunks {
		if !encoder2.VerifyThenAddChunk(chunk) {
			t.Fatal("failed to add valid chunk")
		}
	}

	// Test reconstruction in second encoder
	reconstructed, err := encoder2.ReconstructMessage(messageID)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(reconstructed, msg) {
		t.Fatalf("reconstructed message doesn't match original: got %v, want %v", reconstructed, msg)
	}
}

func TestRlncEncoderEncodeDecodeChunk(t *testing.T) {
	f := field.NewPrimeField(big.NewInt(4_294_967_311))

	config := &RlncEncoderConfig{
		MessageChunkSize:   4,
		NetworkChunkSize:   16,
		ElementsPerChunk:   2,
		MaxCoefficientBits: 16,
		Field:              f,
	}

	encoder, err := NewRlncEncoder(config)
	if err != nil {
		t.Fatal(err)
	}

	msg := []byte("testdata")
	messageID := "test-encode-decode"

	// Generate chunks
	_, err = encoder.GenerateThenAddChunks(messageID, msg)
	if err != nil {
		t.Fatal(err)
	}
	chunks := encoder.GetChunks(messageID)

	// Test encoding and decoding
	originalChunk := chunks[0]

	// Encode the chunk
	encodedExtra := originalChunk.EncodeExtra()
	chunkData := originalChunk.Data()

	// Decode the chunk
	decodedChunk, err := encoder.DecodeChunk(messageID, chunkData, encodedExtra)
	if err != nil {
		t.Fatal(err)
	}

	decodedRlncChunk := decodedChunk.(Chunk)

	// Compare original and decoded chunks
	if !slices.Equal(originalChunk.ChunkData, decodedRlncChunk.ChunkData) {
		t.Fatal("decoded chunk data doesn't match original")
	}

	if len(originalChunk.Coeffs) != len(decodedRlncChunk.Coeffs) {
		t.Fatal("decoded coefficients length doesn't match original")
	}

	// Test that coefficients are equal
	for i, origCoeff := range originalChunk.Coeffs {
		decodedCoeff := decodedRlncChunk.Coeffs[i]
		if !origCoeff.Equal(decodedCoeff) {
			t.Fatalf("coefficient %d doesn't match: original=%v, decoded=%v", i, origCoeff, decodedCoeff)
		}
	}
}

func TestRlncEncoderGetMethods(t *testing.T) {
	f := field.NewPrimeField(big.NewInt(4_294_967_311))

	config := &RlncEncoderConfig{
		MessageChunkSize:   4,
		NetworkChunkSize:   16,
		ElementsPerChunk:   2,
		MaxCoefficientBits: 16,
		Field:              f,
	}

	encoder, err := NewRlncEncoder(config)
	if err != nil {
		t.Fatal(err)
	}

	// Initially no messages
	if len(encoder.GetMessageIDs()) != 0 {
		t.Fatal("expected no message IDs initially")
	}

	msg1 := []byte("test1234")
	msg2 := []byte("abcdefgh")
	messageID1 := "test-msg-1"
	messageID2 := "test-msg-2"

	// Generate chunks for first message
	_, err = encoder.GenerateThenAddChunks(messageID1, msg1)
	if err != nil {
		t.Fatal(err)
	}

	// Generate chunks for second message
	_, err = encoder.GenerateThenAddChunks(messageID2, msg2)
	if err != nil {
		t.Fatal(err)
	}

	// Test GetMessageIDs
	messageIDs := encoder.GetMessageIDs()
	if len(messageIDs) != 2 {
		t.Fatalf("expected 2 message IDs, got %d", len(messageIDs))
	}

	// Test GetChunkCount
	count1 := encoder.GetChunkCount(messageID1)
	count2 := encoder.GetChunkCount(messageID2)
	if count1 != 2 || count2 != 2 {
		t.Fatalf("expected 2 chunks for each message, got %d and %d", count1, count2)
	}

	// Test GetMinChunksForReconstruction
	minChunks1 := encoder.GetMinChunksForReconstruction(messageID1)
	minChunks2 := encoder.GetMinChunksForReconstruction(messageID2)
	if minChunks1 != 2 || minChunks2 != 2 {
		t.Fatalf("expected 2 min chunks for each message, got %d and %d", minChunks1, minChunks2)
	}

	// Test with non-existent message
	if encoder.GetChunkCount("nonexistent") != 0 {
		t.Fatal("expected 0 chunks for nonexistent message")
	}

	if encoder.GetMinChunksForReconstruction("nonexistent") != 0 {
		t.Fatal("expected 0 min chunks for nonexistent message")
	}
}

func TestRlncEncoderInvalidConfig(t *testing.T) {
	f := field.NewPrimeField(big.NewInt(4_294_967_311))

	// Test invalid ElementsPerChunk
	config := &RlncEncoderConfig{
		MessageChunkSize:   7, // Not divisible by ElementsPerChunk (7*8=56 not divisible by 3)
		NetworkChunkSize:   16,
		ElementsPerChunk:   3,
		MaxCoefficientBits: 16,
		Field:              f,
	}

	_, err := NewRlncEncoder(config)
	if err == nil {
		t.Fatal("expected error for invalid ElementsPerChunk configuration")
	}
}

func TestRlncEncoderDefaultConfig(t *testing.T) {
	encoder, err := NewRlncEncoder(nil)
	if err != nil {
		t.Fatal(err)
	}

	// Test with default configuration
	msg := make([]byte, 1024) // Exactly one chunk with default config
	for i := range msg {
		msg[i] = byte(i % 256)
	}

	messageID := "test-default"

	numChunks, err := encoder.GenerateThenAddChunks(messageID, msg)
	if err != nil {
		t.Fatal(err)
	}

	if numChunks != 1 {
		t.Fatalf("expected 1 chunk with default config, got %d", numChunks)
	}

	// Test reconstruction
	reconstructed, err := encoder.ReconstructMessage(messageID)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(reconstructed, msg) {
		t.Fatal("reconstructed message doesn't match original with default config")
	}
}

func TestRlncEncoderBinaryField(t *testing.T) {
	f := field.NewBinaryFieldGF2_32()

	config := &RlncEncoderConfig{
		MessageChunkSize:   8,
		NetworkChunkSize:   9,
		ElementsPerChunk:   2,
		MaxCoefficientBits: 16,
		Field:              f,
	}

	encoder, err := NewRlncEncoder(config)
	if err != nil {
		t.Fatal(err)
	}

	msg := []byte("binary12")
	messageID := "test-binary"

	numChunks, err := encoder.GenerateThenAddChunks(messageID, msg)
	if err != nil {
		t.Fatal(err)
	}

	if numChunks != 1 {
		t.Fatalf("expected 1 chunk, got %d", numChunks)
	}

	// Test reconstruction with binary field
	reconstructed, err := encoder.ReconstructMessage(messageID)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(reconstructed, msg) {
		t.Fatalf("reconstructed message doesn't match original with binary field: got %v, want %v", reconstructed, msg)
	}
}
