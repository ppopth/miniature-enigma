package rs

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethp2p/eth-ec-broadcast/ec/encode"
	"github.com/ethp2p/eth-ec-broadcast/ec/field"
	"github.com/ethp2p/eth-ec-broadcast/pb"
	"github.com/ethp2p/eth-ec-broadcast/pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
)

func TestNewRsEncoder(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		encoder, err := NewRsEncoder(nil)
		if err != nil {
			t.Fatalf("NewRsEncoder failed: %v", err)
		}
		if encoder == nil {
			t.Fatal("encoder is nil")
		}
		// Check default values
		if encoder.config.ParityRatio != 0.5 {
			t.Errorf("expected parity ratio 0.5, got %f", encoder.config.ParityRatio)
		}
	})

	t.Run("custom config", func(t *testing.T) {
		f := field.NewBinaryField(8, big.NewInt(0x11B))
		config := &RsEncoderConfig{
			ParityRatio:      1.0,
			MessageChunkSize: 512,
			NetworkChunkSize: 512,
			ElementsPerChunk: 512,
			Field:            f,
			PrimitiveElement: f.FromBytes([]byte{0x03}),
		}
		encoder, err := NewRsEncoder(config)
		if err != nil {
			t.Fatalf("NewRsEncoder failed: %v", err)
		}
		if encoder.config.ParityRatio != 1.0 {
			t.Errorf("expected parity ratio 1.0, got %f", encoder.config.ParityRatio)
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		tests := []struct {
			name   string
			config *RsEncoderConfig
		}{
			{
				name: "negative parity ratio",
				config: &RsEncoderConfig{
					ParityRatio:      -0.5,
					MessageChunkSize: 1024,
					NetworkChunkSize: 1024,
					ElementsPerChunk: 1024,
					Field:            field.NewBinaryField(8, big.NewInt(0x11B)),
				},
			},
			{
				name: "zero message chunk size",
				config: &RsEncoderConfig{
					ParityRatio:      0.5,
					MessageChunkSize: 0,
					NetworkChunkSize: 1024,
					ElementsPerChunk: 1024,
					Field:            field.NewBinaryField(8, big.NewInt(0x11B)),
				},
			},
			{
				name: "indivisible elements per chunk",
				config: &RsEncoderConfig{
					ParityRatio:      0.5,
					MessageChunkSize: 1024,
					NetworkChunkSize: 1024,
					ElementsPerChunk: 1000, // 8*1024 = 8192 is not divisible by 1000
					Field:            field.NewBinaryField(8, big.NewInt(0x11B)),
					PrimitiveElement: field.NewBinaryField(8, big.NewInt(0x11B)).FromBytes([]byte{0x03}),
				},
			},
			{
				name: "missing primitive element",
				config: &RsEncoderConfig{
					ParityRatio:      0.5,
					MessageChunkSize: 1024,
					NetworkChunkSize: 1024,
					ElementsPerChunk: 1024,
					Field:            field.NewBinaryField(8, big.NewInt(0x11B)),
					// PrimitiveElement is missing
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := NewRsEncoder(tt.config)
				if err == nil {
					t.Error("expected error but got nil")
				}
			})
		}
	})
}

func TestGenerateThenAddChunks(t *testing.T) {
	encoder, err := NewRsEncoder(nil)
	if err != nil {
		t.Fatalf("NewRsEncoder failed: %v", err)
	}

	t.Run("basic encoding", func(t *testing.T) {
		messageID := "test-msg-1"
		message := make([]byte, 2048) // 2 chunks
		for i := range message {
			message[i] = byte(i % 256)
		}

		totalChunks, err := encoder.GenerateThenAddChunks(messageID, message)
		if err != nil {
			t.Fatalf("GenerateThenAddChunks failed: %v", err)
		}

		// With 50% parity ratio and 2 data chunks, we should get 3 total chunks
		expectedTotal := 3
		if totalChunks != expectedTotal {
			t.Errorf("expected %d total chunks, got %d", expectedTotal, totalChunks)
		}

		// Check chunk count is stored
		count := encoder.GetChunkCount(messageID)
		if count != expectedTotal {
			t.Errorf("expected %d chunks stored, got %d", expectedTotal, count)
		}
	})

	t.Run("message not multiple of chunk size", func(t *testing.T) {
		messageID := "test-msg-2"
		message := make([]byte, 1500) // Not a multiple of 1024

		_, err := encoder.GenerateThenAddChunks(messageID, message)
		if err == nil {
			t.Error("expected error for message not multiple of chunk size")
		}
	})

	t.Run("empty message", func(t *testing.T) {
		messageID := "test-msg-3"
		message := make([]byte, 0)

		_, err := encoder.GenerateThenAddChunks(messageID, message)
		if err == nil {
			t.Error("expected error for empty message")
		}
	})

	t.Run("multiple parity chunks", func(t *testing.T) {
		f := field.NewBinaryField(8, big.NewInt(0x11B))
		config := &RsEncoderConfig{
			ParityRatio:      2.0, // 200% redundancy
			MessageChunkSize: 512,
			NetworkChunkSize: 512,
			ElementsPerChunk: 512,
			Field:            f,
			PrimitiveElement: f.FromBytes([]byte{0x03}),
		}
		encoder, err := NewRsEncoder(config)
		if err != nil {
			t.Fatalf("NewRsEncoder failed: %v", err)
		}

		messageID := "test-msg-4"
		message := make([]byte, 1024) // 2 data chunks
		for i := range message {
			message[i] = byte(i % 256)
		}

		totalChunks, err := encoder.GenerateThenAddChunks(messageID, message)
		if err != nil {
			t.Fatalf("GenerateThenAddChunks failed: %v", err)
		}

		// With 200% parity ratio and 2 data chunks, we should get 6 total chunks (2 + 4)
		expectedTotal := 6
		if totalChunks != expectedTotal {
			t.Errorf("expected %d total chunks, got %d", expectedTotal, totalChunks)
		}
	})
}

func TestVerifyThenAddChunk(t *testing.T) {
	encoder, err := NewRsEncoder(nil)
	if err != nil {
		t.Fatalf("NewRsEncoder failed: %v", err)
	}

	t.Run("add valid chunk", func(t *testing.T) {
		chunk := Chunk{
			MessageID:  "test-msg",
			Index:      0,
			ChunkData:  make([]byte, 1024),
			ChunkCount: 2,
		}

		result := encoder.VerifyThenAddChunk(pubsub.PeerSend{}, chunk)
		if !result {
			t.Error("expected chunk to be added successfully")
		}
	})

	t.Run("reject duplicate chunk", func(t *testing.T) {
		// Use fresh encoder for this test
		freshEncoder, err := NewRsEncoder(nil)
		if err != nil {
			t.Fatalf("NewRsEncoder failed: %v", err)
		}

		chunk := Chunk{
			MessageID:  "test-msg-duplicate",
			Index:      0,
			ChunkData:  make([]byte, 1024),
			ChunkCount: 2,
		}

		// Add first time
		result := freshEncoder.VerifyThenAddChunk(pubsub.PeerSend{}, chunk)
		if !result {
			t.Error("expected first chunk to be added successfully")
		}

		// Try to add again
		result = freshEncoder.VerifyThenAddChunk(pubsub.PeerSend{}, chunk)
		if result {
			t.Error("expected duplicate chunk to be rejected")
		}
	})

	t.Run("reject invalid chunk count", func(t *testing.T) {
		chunk := Chunk{
			MessageID:  "test-msg-2",
			Index:      0,
			ChunkData:  make([]byte, 1024),
			ChunkCount: 0,
		}

		result := encoder.VerifyThenAddChunk(pubsub.PeerSend{}, chunk)
		if result {
			t.Error("expected chunk with invalid count to be rejected")
		}
	})

	t.Run("reject inconsistent chunk count", func(t *testing.T) {
		chunk1 := Chunk{
			MessageID:  "test-msg-3",
			Index:      0,
			ChunkData:  make([]byte, 1024),
			ChunkCount: 2,
		}

		result := encoder.VerifyThenAddChunk(pubsub.PeerSend{}, chunk1)
		if !result {
			t.Error("expected first chunk to be added successfully")
		}

		chunk2 := Chunk{
			MessageID:  "test-msg-3",
			Index:      1,
			ChunkData:  make([]byte, 1024),
			ChunkCount: 3, // Different chunk count
		}

		result = encoder.VerifyThenAddChunk(pubsub.PeerSend{}, chunk2)
		if result {
			t.Error("expected chunk with inconsistent count to be rejected")
		}
	})

	t.Run("reject invalid index", func(t *testing.T) {
		tests := []struct {
			name  string
			index int
		}{
			{"negative index", -1},
			{"index too large", 10}, // With chunkCount=2 and 50% parity, max index is 2
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				chunk := Chunk{
					MessageID:  fmt.Sprintf("test-msg-%s", tt.name),
					Index:      tt.index,
					ChunkData:  make([]byte, 1024),
					ChunkCount: 2,
				}

				result := encoder.VerifyThenAddChunk(pubsub.PeerSend{}, chunk)
				if result {
					t.Error("expected chunk with invalid index to be rejected")
				}
			})
		}
	})
}

func TestEmitChunk(t *testing.T) {
	t.Run("emit chunks in round-robin", func(t *testing.T) {
		encoder, err := NewRsEncoder(nil)
		if err != nil {
			t.Fatalf("NewRsEncoder failed: %v", err)
		}

		messageID := "test-msg"
		// Add chunks in order
		for i := 0; i < 3; i++ {
			chunk := Chunk{
				MessageID:  messageID,
				Index:      i,
				ChunkData:  []byte{byte(i)},
				ChunkCount: 2,
			}
			encoder.VerifyThenAddChunk(pubsub.PeerSend{}, chunk)
		}

		// Emissions should select the earliest chunk with lowest emit count
		// Expected: 0, 1, 2, 0, 1, 2, ... (round-robin)
		expectedIndices := []int{0, 1, 2, 0, 1, 2}
		for i, expected := range expectedIndices {
			emitted, err := encoder.EmitChunk(peer.ID(""), messageID)
			if err != nil {
				t.Fatalf("EmitChunk %d failed: %v", i+1, err)
			}
			if emitted.(Chunk).Index != expected {
				t.Errorf("emission %d: expected chunk index %d, got %d", i+1, expected, emitted.(Chunk).Index)
			}
		}
	})

	t.Run("emit from non-existent message", func(t *testing.T) {
		encoder, err := NewRsEncoder(nil)
		if err != nil {
			t.Fatalf("NewRsEncoder failed: %v", err)
		}

		_, err = encoder.EmitChunk(peer.ID(""), "non-existent")
		if err == nil {
			t.Error("expected error for non-existent message")
		}
	})
}

func TestReconstructMessage(t *testing.T) {
	t.Run("reconstruct from all data chunks", func(t *testing.T) {
		// First encoder to generate chunks
		encoder1, err := NewRsEncoder(nil)
		if err != nil {
			t.Fatalf("NewRsEncoder failed: %v", err)
		}
		messageID := "test-msg"
		original := make([]byte, 2048) // 2 chunks
		for i := range original {
			original[i] = byte(i % 256)
		}

		// Generate chunks
		totalChunks, err := encoder1.GenerateThenAddChunks(messageID, original)
		if err != nil {
			t.Fatalf("GenerateThenAddChunks failed: %v", err)
		}

		// Extract all chunks from first encoder
		allChunks := make([]Chunk, 0)
		encoder1.mutex.Lock()
		for _, chunk := range encoder1.chunks[messageID] {
			allChunks = append(allChunks, chunk)
		}
		encoder1.mutex.Unlock()

		// Second encoder to simulate receiving chunks
		encoder2, err := NewRsEncoder(nil)
		if err != nil {
			t.Fatalf("NewRsEncoder failed: %v", err)
		}

		// Add all chunks to second encoder
		for _, chunk := range allChunks {
			encoder2.VerifyThenAddChunk(pubsub.PeerSend{}, chunk)
		}

		// Reconstruct using second encoder
		reconstructed, err := encoder2.ReconstructMessage(messageID)
		if err != nil {
			t.Fatalf("ReconstructMessage failed: %v", err)
		}

		if !bytes.Equal(original, reconstructed) {
			t.Fatalf("reconstructed message doesn't match original (total chunks: %d)", totalChunks)
		}
	})

	t.Run("reconstruct with insufficient chunks", func(t *testing.T) {
		// First encoder to generate chunks
		encoder1, err := NewRsEncoder(nil)
		if err != nil {
			t.Fatalf("NewRsEncoder failed: %v", err)
		}
		messageID := "test-msg-3"
		original := make([]byte, 2048) // 2 chunks
		for i := range original {
			original[i] = byte(i % 256)
		}

		// Generate chunks
		_, err = encoder1.GenerateThenAddChunks(messageID, original)
		if err != nil {
			t.Fatalf("GenerateThenAddChunks failed: %v", err)
		}

		// Extract all chunks from first encoder
		allChunks := make([]Chunk, 0)
		encoder1.mutex.Lock()
		for _, chunk := range encoder1.chunks[messageID] {
			allChunks = append(allChunks, chunk)
		}
		encoder1.mutex.Unlock()

		// Second encoder to simulate receiving insufficient chunks
		encoder2, err := NewRsEncoder(nil)
		if err != nil {
			t.Fatalf("NewRsEncoder failed: %v", err)
		}

		// Add only one chunk (insufficient for reconstruction of 2 data chunks)
		if len(allChunks) > 0 {
			encoder2.VerifyThenAddChunk(pubsub.PeerSend{}, allChunks[0])
		}

		// Should fail with insufficient chunks
		_, err = encoder2.ReconstructMessage(messageID)
		if err == nil {
			t.Fatal("expected error for insufficient chunks")
		}
	})

	t.Run("reconstruct non-existent message", func(t *testing.T) {
		encoder4, err := NewRsEncoder(nil)
		if err != nil {
			t.Fatalf("NewRsEncoder failed: %v", err)
		}
		_, err = encoder4.ReconstructMessage("non-existent")
		if err == nil {
			t.Error("expected error for non-existent message")
		}
	})
}

func TestDecodeChunk(t *testing.T) {
	encoder, err := NewRsEncoder(nil)
	if err != nil {
		t.Fatalf("NewRsEncoder failed: %v", err)
	}

	t.Run("decode valid chunk", func(t *testing.T) {
		messageID := "test-msg"
		data := make([]byte, 1024)
		index := uint32(1)
		chunkCount := uint32(3)

		rsExtra := &pb.RsExtra{
			Index:      &index,
			ChunkCount: &chunkCount,
			Extra:      []byte("test-extra"),
		}
		extraData, _ := rsExtra.Marshal()

		chunk, err := encoder.DecodeChunk(messageID, data, extraData)
		if err != nil {
			t.Fatalf("DecodeChunk failed: %v", err)
		}

		rsChunk := chunk.(Chunk)
		if rsChunk.Index != 1 {
			t.Errorf("expected index 1, got %d", rsChunk.Index)
		}
		if rsChunk.ChunkCount != 3 {
			t.Errorf("expected chunk count 3, got %d", rsChunk.ChunkCount)
		}
		if !bytes.Equal(rsChunk.Extra, []byte("test-extra")) {
			t.Error("extra data doesn't match")
		}
	})

	t.Run("decode with nil index", func(t *testing.T) {
		messageID := "test-msg"
		data := make([]byte, 1024)
		chunkCount := uint32(3)

		rsExtra := &pb.RsExtra{
			ChunkCount: &chunkCount,
		}
		extraData, _ := rsExtra.Marshal()

		_, err := encoder.DecodeChunk(messageID, data, extraData)
		if err == nil {
			t.Error("expected error for nil index")
		}
	})

	t.Run("decode with nil chunk count", func(t *testing.T) {
		messageID := "test-msg"
		data := make([]byte, 1024)
		index := uint32(1)

		rsExtra := &pb.RsExtra{
			Index: &index,
		}
		extraData, _ := rsExtra.Marshal()

		_, err := encoder.DecodeChunk(messageID, data, extraData)
		if err == nil {
			t.Error("expected error for nil chunk count")
		}
	})

	t.Run("decode with invalid extra data", func(t *testing.T) {
		messageID := "test-msg"
		data := make([]byte, 1024)
		extraData := []byte("invalid protobuf")

		_, err := encoder.DecodeChunk(messageID, data, extraData)
		if err == nil {
			t.Error("expected error for invalid extra data")
		}
	})
}

func TestChunkInterface(t *testing.T) {
	chunk := Chunk{
		MessageID:  "test",
		Index:      1,
		ChunkData:  []byte("test data"),
		ChunkCount: 2,
		Extra:      []byte("extra"),
	}

	// Test Data() method
	if !bytes.Equal(chunk.Data(), []byte("test data")) {
		t.Error("Data() method doesn't return correct data")
	}

	// Test EncodeExtra() method
	extra := chunk.EncodeExtra()
	rsExtra := &pb.RsExtra{}
	if err := rsExtra.Unmarshal(extra); err != nil {
		t.Fatalf("Failed to unmarshal extra data: %v", err)
	}
	if *rsExtra.Index != 1 {
		t.Errorf("expected index 1, got %d", *rsExtra.Index)
	}
	if *rsExtra.ChunkCount != 2 {
		t.Errorf("expected chunk count 2, got %d", *rsExtra.ChunkCount)
	}
	if !bytes.Equal(rsExtra.Extra, []byte("extra")) {
		t.Error("extra data doesn't match")
	}

	// Ensure Chunk implements encode.Chunk interface
	var _ encode.Chunk = chunk
}

func TestGetters(t *testing.T) {
	encoder, err := NewRsEncoder(nil)
	if err != nil {
		t.Fatalf("NewRsEncoder failed: %v", err)
	}

	// Generate some test data
	messageID1 := "msg1"
	messageID2 := "msg2"
	message := make([]byte, 2048)

	encoder.GenerateThenAddChunks(messageID1, message)
	encoder.GenerateThenAddChunks(messageID2, message)

	t.Run("GetMessageIDs", func(t *testing.T) {
		ids := encoder.GetMessageIDs()
		if len(ids) != 2 {
			t.Errorf("expected 2 message IDs, got %d", len(ids))
		}

		// Check both IDs are present
		found := make(map[string]bool)
		for _, id := range ids {
			found[id] = true
		}
		if !found[messageID1] || !found[messageID2] {
			t.Error("not all message IDs found")
		}
	})

	t.Run("GetChunkCount", func(t *testing.T) {
		count := encoder.GetChunkCount(messageID1)
		if count != 3 { // 2 data + 1 parity with 50% ratio
			t.Errorf("expected 3 chunks, got %d", count)
		}

		count = encoder.GetChunkCount("non-existent")
		if count != 0 {
			t.Errorf("expected 0 chunks for non-existent message, got %d", count)
		}
	})

	t.Run("GetMinChunksForReconstruction", func(t *testing.T) {
		min := encoder.GetMinChunksForReconstruction(messageID1)
		if min != 2 { // 2 data chunks
			t.Errorf("expected 2 minimum chunks, got %d", min)
		}

		min = encoder.GetMinChunksForReconstruction("non-existent")
		if min != 0 {
			t.Errorf("expected 0 for non-existent message, got %d", min)
		}
	})
}

func TestMatrixGeneration(t *testing.T) {
	encoder, err := NewRsEncoder(nil)
	if err != nil {
		t.Fatalf("NewRsEncoder failed: %v", err)
	}

	t.Run("verify systematic property", func(t *testing.T) {
		chunkCount := 3
		parityCount := 2
		matrix := encoder.generateEncodingMatrix(chunkCount, parityCount)

		// Check dimensions
		if len(matrix) != chunkCount+parityCount {
			t.Errorf("expected %d rows, got %d", chunkCount+parityCount, len(matrix))
		}
		for i, row := range matrix {
			if len(row) != chunkCount {
				t.Errorf("row %d: expected %d columns, got %d", i, chunkCount, len(row))
			}
		}

		// Check identity portion (first k rows should form identity matrix)
		field := encoder.config.Field
		one := field.One()
		for i := 0; i < chunkCount; i++ {
			for j := 0; j < chunkCount; j++ {
				if i == j {
					if !matrix[i][j].Equal(one) {
						t.Errorf("expected identity matrix at [%d][%d], got non-one", i, j)
					}
				} else {
					if !matrix[i][j].IsZero() {
						t.Errorf("expected identity matrix at [%d][%d], got non-zero", i, j)
					}
				}
			}
		}

		// Check parity portion exists and is non-trivial
		hasNonZero := false
		for i := chunkCount; i < chunkCount+parityCount; i++ {
			for j := 0; j < chunkCount; j++ {
				if !matrix[i][j].IsZero() {
					hasNonZero = true
					break
				}
			}
		}
		if !hasNonZero {
			t.Fatal("parity matrix is all zeros")
		}
	})
}

func TestReedSolomonCorrectness(t *testing.T) {
	t.Run("end-to-end erasure recovery", func(t *testing.T) {
		testCases := []struct {
			name          string
			dataChunks    int
			parityRatio   float64
			erasures      []int // indices to erase
			shouldSucceed bool
		}{
			{
				name:          "no erasures",
				dataChunks:    4,
				parityRatio:   0.5, // 2 parity chunks
				erasures:      []int{},
				shouldSucceed: true,
			},
			{
				name:          "erase one data chunk",
				dataChunks:    4,
				parityRatio:   0.5,
				erasures:      []int{1},
				shouldSucceed: true,
			},
			{
				name:          "erase one parity chunk",
				dataChunks:    4,
				parityRatio:   0.5,
				erasures:      []int{4}, // First parity chunk
				shouldSucceed: true,
			},
			{
				name:          "erase maximum allowed chunks",
				dataChunks:    4,
				parityRatio:   0.5,
				erasures:      []int{1, 5}, // 1 data + 1 parity
				shouldSucceed: true,
			},
			{
				name:          "erase too many chunks",
				dataChunks:    4,
				parityRatio:   0.5,
				erasures:      []int{0, 1, 2}, // 3 chunks erased, need at least 4
				shouldSucceed: false,
			},
			{
				name:          "high redundancy - erase many chunks",
				dataChunks:    3,
				parityRatio:   2.0,                     // 6 parity chunks
				erasures:      []int{0, 2, 3, 4, 5, 7}, // Erase 6 out of 9 chunks
				shouldSucceed: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create config for this test case
				f := field.NewBinaryField(8, big.NewInt(0x11B))
				config := &RsEncoderConfig{
					ParityRatio:      tc.parityRatio,
					MessageChunkSize: 512,
					NetworkChunkSize: 512,
					ElementsPerChunk: 512,
					Field:            f,
					PrimitiveElement: f.FromBytes([]byte{0x03}),
				}

				// First encoder to generate chunks
				encoder1, err := NewRsEncoder(config)
				if err != nil {
					t.Fatalf("NewRsEncoder failed: %v", err)
				}

				messageID := fmt.Sprintf("test-%s", tc.name)
				message := make([]byte, tc.dataChunks*512)
				for i := range message {
					message[i] = byte(i % 256)
				}

				// Generate chunks
				totalChunks, err := encoder1.GenerateThenAddChunks(messageID, message)
				if err != nil {
					t.Fatalf("GenerateThenAddChunks failed: %v", err)
				}

				fmt.Printf("totalChunks %d\n", totalChunks)

				// Extract all chunks
				allChunks := make([]Chunk, 0)
				encoder1.mutex.Lock()
				for _, chunk := range encoder1.chunks[messageID] {
					allChunks = append(allChunks, chunk)
				}
				encoder1.mutex.Unlock()

				// Second encoder to simulate erasures
				encoder2, err := NewRsEncoder(config)
				if err != nil {
					t.Fatalf("NewRsEncoder failed: %v", err)
				}

				// Add chunks except the erased ones
				erasureSet := make(map[int]bool)
				for _, idx := range tc.erasures {
					erasureSet[idx] = true
				}

				availableCount := 0
				for _, chunk := range allChunks {
					if !erasureSet[chunk.Index] {
						encoder2.VerifyThenAddChunk(pubsub.PeerSend{}, chunk)
						availableCount++
					}
				}

				// Try to reconstruct
				reconstructed, err := encoder2.ReconstructMessage(messageID)

				if tc.shouldSucceed {
					if err != nil {
						t.Fatalf("ReconstructMessage failed: %v (available: %d, total: %d, erased: %v)",
							err, availableCount, totalChunks, tc.erasures)
					}
					if !bytes.Equal(message, reconstructed) {
						t.Fatalf("reconstructed message doesn't match original (available: %d, total: %d)",
							availableCount, totalChunks)
					}
				} else {
					if err == nil {
						t.Fatalf("expected reconstruction to fail but it succeeded (available: %d, total: %d)",
							availableCount, totalChunks)
					}
				}
			})
		}
	})

	t.Run("verify MDS property", func(t *testing.T) {
		// Test that ANY k chunks can reconstruct the message (MDS property)
		f := field.NewBinaryField(8, big.NewInt(0x11B))
		config := &RsEncoderConfig{
			ParityRatio:      1.0, // 100% redundancy for thorough testing
			MessageChunkSize: 256,
			NetworkChunkSize: 256,
			ElementsPerChunk: 256,
			Field:            f,
			PrimitiveElement: f.FromBytes([]byte{0x03}),
		}

		encoder1, err := NewRsEncoder(config)
		if err != nil {
			t.Fatalf("NewRsEncoder failed: %v", err)
		}

		messageID := "mds-test"
		message := make([]byte, 512) // 2 data chunks
		for i := range message {
			message[i] = byte(i % 256)
		}

		// Generate chunks (2 data + 2 parity = 4 total)
		_, err = encoder1.GenerateThenAddChunks(messageID, message)
		if err != nil {
			t.Fatalf("GenerateThenAddChunks failed: %v", err)
		}

		// Extract all chunks
		allChunks := make([]Chunk, 0)
		encoder1.mutex.Lock()
		for _, chunk := range encoder1.chunks[messageID] {
			allChunks = append(allChunks, chunk)
		}
		encoder1.mutex.Unlock()

		// Test all possible combinations of k=2 chunks from n=4 total
		dataChunks := 2
		combinations := [][]int{
			{0, 1}, // Both data chunks
			{0, 2}, // Data chunk 0 + parity chunk 0
			{0, 3}, // Data chunk 0 + parity chunk 1
			{1, 2}, // Data chunk 1 + parity chunk 0
			{1, 3}, // Data chunk 1 + parity chunk 1
			{2, 3}, // Both parity chunks
		}

		for i, combo := range combinations {
			t.Run(fmt.Sprintf("combination_%d_%v", i, combo), func(t *testing.T) {
				encoder2, err := NewRsEncoder(config)
				if err != nil {
					t.Fatalf("NewRsEncoder failed: %v", err)
				}

				// Add only the chunks in this combination
				addedCount := 0
				for _, chunk := range allChunks {
					for _, idx := range combo {
						if chunk.Index == idx {
							encoder2.VerifyThenAddChunk(pubsub.PeerSend{}, chunk)
							addedCount++
							break
						}
					}
				}

				if addedCount != dataChunks {
					t.Fatalf("expected to add %d chunks, added %d", dataChunks, addedCount)
				}

				// Should be able to reconstruct from any k chunks
				reconstructed, err := encoder2.ReconstructMessage(messageID)
				if err != nil {
					t.Fatalf("ReconstructMessage failed for combination %v: %v", combo, err)
				}

				if !bytes.Equal(message, reconstructed) {
					t.Fatalf("reconstructed message doesn't match original for combination %v", combo)
				}
			})
		}
	})

	t.Run("verify linearity property", func(t *testing.T) {
		// Test that Reed-Solomon encoding is linear: RS(a + b) = RS(a) + RS(b)
		encoder, err := NewRsEncoder(nil)
		if err != nil {
			t.Fatalf("NewRsEncoder failed: %v", err)
		}

		// Create two messages
		messageA := make([]byte, 1024)
		messageB := make([]byte, 1024)
		messageSum := make([]byte, 1024)

		for i := range messageA {
			messageA[i] = byte(i % 13)
			messageB[i] = byte((i * 7) % 17)
			messageSum[i] = messageA[i] ^ messageB[i] // XOR for GF(2^8)
		}

		// Encode all three messages
		_, err = encoder.GenerateThenAddChunks("msgA", messageA)
		if err != nil {
			t.Fatalf("Failed to encode messageA: %v", err)
		}

		_, err = encoder.GenerateThenAddChunks("msgB", messageB)
		if err != nil {
			t.Fatalf("Failed to encode messageB: %v", err)
		}

		_, err = encoder.GenerateThenAddChunks("msgSum", messageSum)
		if err != nil {
			t.Fatalf("Failed to encode messageSum: %v", err)
		}

		// Extract chunks and verify linearity
		encoder.mutex.Lock()
		chunksA := encoder.chunks["msgA"]
		chunksB := encoder.chunks["msgB"]
		chunksSum := encoder.chunks["msgSum"]
		encoder.mutex.Unlock()

		// Check that each chunk satisfies: Sum[i] = A[i] ⊕ B[i]
		for idx := range chunksSum {
			if _, hasA := chunksA[idx]; !hasA {
				continue
			}
			if _, hasB := chunksB[idx]; !hasB {
				continue
			}

			dataA := chunksA[idx].ChunkData
			dataB := chunksB[idx].ChunkData
			dataSum := chunksSum[idx].ChunkData

			if len(dataA) != len(dataB) || len(dataA) != len(dataSum) {
				t.Fatalf("chunk %d: length mismatch", idx)
			}

			// Verify XOR property
			for j := range dataA {
				expected := dataA[j] ^ dataB[j]
				if dataSum[j] != expected {
					t.Fatalf("chunk %d, byte %d: linearity violated. Expected %d, got %d",
						idx, j, expected, dataSum[j])
				}
			}
		}
	})
}

// TestReedSolomonWithPrimeField tests Reed-Solomon encoder with a prime field
func TestReedSolomonWithPrimeField(t *testing.T) {
	// Use a moderate-sized prime for testing
	prime := big.NewInt(65537) // 2^16 + 1, a well-known Fermat prime
	primeField := field.NewPrimeField(prime)

	// Find a primitive element for the prime field
	// For GF(65537), 3 is a primitive element
	primitiveElement := primeField.FromBytes(big.NewInt(3).Bytes())

	config := &RsEncoderConfig{
		ParityRatio:      0.5, // 50% redundancy
		MessageChunkSize: 128, // Smaller chunks for prime field testing
		NetworkChunkSize: 256, // Larger network chunks to accommodate prime field elements
		ElementsPerChunk: 64,  // Number of field elements per chunk
		Field:            primeField,
		PrimitiveElement: primitiveElement,
	}

	encoder, err := NewRsEncoder(config)
	if err != nil {
		t.Fatalf("Failed to create RS encoder with prime field: %v", err)
	}

	t.Run("basic encoding and reconstruction", func(t *testing.T) {
		// Create test message (multiple of chunk size)
		message := make([]byte, 256) // 2 chunks
		for i := range message {
			message[i] = byte(i % 256)
		}

		messageID := "prime-field-test"
		totalChunks, err := encoder.GenerateThenAddChunks(messageID, message)
		if err != nil {
			t.Fatalf("GenerateThenAddChunks failed: %v", err)
		}

		expectedDataChunks := len(message) / config.MessageChunkSize
		expectedParityChunks := int(float64(expectedDataChunks) * config.ParityRatio)
		if expectedParityChunks == 0 {
			expectedParityChunks = 1
		}
		expectedTotal := expectedDataChunks + expectedParityChunks

		if totalChunks != expectedTotal {
			t.Fatalf("Expected %d total chunks, got %d", expectedTotal, totalChunks)
		}

		// Test reconstruction with all data chunks
		reconstructed, err := encoder.ReconstructMessage(messageID)
		if err != nil {
			t.Fatalf("ReconstructMessage failed: %v", err)
		}

		if !bytes.Equal(message, reconstructed) {
			t.Errorf("Reconstructed message doesn't match original")
		}
	})

	t.Run("erasure recovery with prime field", func(t *testing.T) {
		// Create a larger message for more thorough testing
		message := make([]byte, 512) // 4 chunks
		for i := range message {
			message[i] = byte((i*17 + 42) % 256) // More varied pattern
		}

		messageID := "prime-field-erasure-test"
		totalChunks, err := encoder.GenerateThenAddChunks(messageID, message)
		if err != nil {
			t.Fatalf("GenerateThenAddChunks failed: %v", err)
		}

		// Remove one data chunk and one parity chunk to test erasure recovery
		// First, get all chunks
		allChunks := make([]Chunk, 0, totalChunks)
		for i := 0; i < totalChunks; i++ {
			chunk, err := encoder.EmitChunk(peer.ID(""), messageID)
			if err != nil {
				t.Fatalf("EmitChunk failed for index %d: %v", i, err)
			}
			allChunks = append(allChunks, chunk.(Chunk))
		}

		// Create a new encoder to simulate receiving chunks over network
		encoder2, err := NewRsEncoder(config)
		if err != nil {
			t.Fatalf("Failed to create second encoder: %v", err)
		}

		// Add all chunks except index 1 (data) and last chunk (parity)
		for i, chunk := range allChunks {
			if i == 1 || i == len(allChunks)-1 {
				continue // Skip these chunks to simulate erasures
			}
			if !encoder2.VerifyThenAddChunk(pubsub.PeerSend{}, chunk) {
				t.Fatalf("Failed to add chunk %d", i)
			}
		}

		// Should still be able to reconstruct
		reconstructed, err := encoder2.ReconstructMessage(messageID)
		if err != nil {
			t.Fatalf("ReconstructMessage failed with erasures: %v", err)
		}

		if !bytes.Equal(message, reconstructed) {
			t.Errorf("Reconstructed message with erasures doesn't match original")
		}
	})
}

// Mock verifier implementation for testing
type mockVerifier struct {
	verifyResult bool
	extraData    []byte
}

func (m *mockVerifier) Verify(chunk *Chunk) bool {
	return m.verifyResult
}

func (m *mockVerifier) GenerateExtra(polynomials [][]field.Element, evalPoint field.Element) ([]byte, error) {
	return m.extraData, nil
}

// TestChunkVerifierIntegration tests the ChunkVerifier interface integration
func TestChunkVerifierIntegration(t *testing.T) {

	t.Run("verifier accepts valid chunks", func(t *testing.T) {
		verifier := &mockVerifier{
			verifyResult: true,
			extraData:    []byte("verification-data"),
		}

		config := DefaultRsEncoderConfig()
		config.ChunkVerifier = verifier

		encoder, err := NewRsEncoder(config)
		if err != nil {
			t.Fatalf("NewRsEncoder failed: %v", err)
		}

		message := make([]byte, 1024)
		for i := range message {
			message[i] = byte(i % 256)
		}

		messageID := "verifier-test"
		totalChunks, err := encoder.GenerateThenAddChunks(messageID, message)
		if err != nil {
			t.Fatalf("GenerateThenAddChunks failed: %v", err)
		}

		// Check that chunks have extra data
		for i := 0; i < totalChunks; i++ {
			chunk, err := encoder.EmitChunk(peer.ID(""), messageID)
			if err != nil {
				t.Fatalf("EmitChunk failed: %v", err)
			}
			rsChunk := chunk.(Chunk)
			if len(rsChunk.Extra) == 0 {
				t.Errorf("Chunk %d missing extra data", i)
			}
			if string(rsChunk.Extra) != "verification-data" {
				t.Errorf("Chunk %d has incorrect extra data: got %s, want verification-data",
					i, string(rsChunk.Extra))
			}
		}
	})

	t.Run("verifier rejects invalid chunks", func(t *testing.T) {
		verifier := &mockVerifier{
			verifyResult: false, // Reject all chunks
			extraData:    []byte("verification-data"),
		}

		config := DefaultRsEncoderConfig()
		config.ChunkVerifier = verifier

		encoder, err := NewRsEncoder(config)
		if err != nil {
			t.Fatalf("NewRsEncoder failed: %v", err)
		}

		// Create a chunk manually
		chunk := Chunk{
			MessageID:  "test",
			Index:      0,
			ChunkData:  []byte("test-data"),
			ChunkCount: 2,
			Extra:      []byte("some-extra"),
		}

		// Verifier should reject this chunk
		if encoder.VerifyThenAddChunk(pubsub.PeerSend{}, chunk) {
			t.Error("Expected verifier to reject chunk, but it was accepted")
		}

		// Chunk should not be stored
		if encoder.GetChunkCount("test") != 0 {
			t.Error("Chunk was stored despite being rejected by verifier")
		}
	})

	t.Run("encoder works without verifier", func(t *testing.T) {
		// Test that encoder still works when no verifier is configured
		config := DefaultRsEncoderConfig()
		config.ChunkVerifier = nil // No verifier

		encoder, err := NewRsEncoder(config)
		if err != nil {
			t.Fatalf("NewRsEncoder failed: %v", err)
		}

		message := make([]byte, 1024)
		for i := range message {
			message[i] = byte(i % 256)
		}

		messageID := "no-verifier-test"
		totalChunks, err := encoder.GenerateThenAddChunks(messageID, message)
		if err != nil {
			t.Fatalf("GenerateThenAddChunks failed: %v", err)
		}

		// Should be able to reconstruct without verifier
		reconstructed, err := encoder.ReconstructMessage(messageID)
		if err != nil {
			t.Fatalf("ReconstructMessage failed: %v", err)
		}

		if !bytes.Equal(message, reconstructed) {
			t.Error("Reconstruction failed without verifier")
		}

		// Chunks should not have extra data
		for i := 0; i < totalChunks; i++ {
			chunk, err := encoder.EmitChunk(peer.ID(""), messageID)
			if err != nil {
				t.Fatalf("EmitChunk failed: %v", err)
			}
			rsChunk := chunk.(Chunk)
			if len(rsChunk.Extra) != 0 {
				t.Errorf("Chunk %d has unexpected extra data when no verifier configured", i)
			}
		}
	})
}

// Benchmarks

func BenchmarkRsGenerateThenAddChunks(b *testing.B) {
	// Standard binary field for RS
	f := field.NewBinaryField(8, big.NewInt(0x11B))
	primitiveElement := f.FromBytes([]byte{0x03})

	config := &RsEncoderConfig{
		ParityRatio:      0.5,  // 50% redundancy
		MessageChunkSize: 1024, // 1KB chunks
		NetworkChunkSize: 1030,
		ElementsPerChunk: 1024 / len(f.One().Bytes()),
		Field:            f,
		PrimitiveElement: primitiveElement,
	}

	encoder, err := NewRsEncoder(config)
	if err != nil {
		b.Fatal(err)
	}

	// Create test messages
	messages := make([][]byte, 10)
	for i := 0; i < 10; i++ {
		messages[i] = make([]byte, 16*1024) // 16KB messages
		for j := range messages[i] {
			messages[i][j] = byte(j % 256)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg := messages[i%10]
		messageID := fmt.Sprintf("msg-%d", i)
		_, err := encoder.GenerateThenAddChunks(messageID, msg)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRsEmitChunk(b *testing.B) {
	f := field.NewBinaryField(8, big.NewInt(0x11B))
	primitiveElement := f.FromBytes([]byte{0x03})

	config := &RsEncoderConfig{
		ParityRatio:      0.5,
		MessageChunkSize: 1024,
		NetworkChunkSize: 1030,
		ElementsPerChunk: 1024 / len(f.One().Bytes()),
		Field:            f,
		PrimitiveElement: primitiveElement,
	}

	encoder, err := NewRsEncoder(config)
	if err != nil {
		b.Fatal(err)
	}

	// Prepare message
	message := make([]byte, 16*1024)
	for i := range message {
		message[i] = byte(i % 256)
	}

	messageID := "bench-msg"
	_, err = encoder.GenerateThenAddChunks(messageID, message)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := encoder.EmitChunk(peer.ID(""), messageID)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRsVerifyThenAddChunk(b *testing.B) {
	f := field.NewBinaryField(8, big.NewInt(0x11B))
	primitiveElement := f.FromBytes([]byte{0x03})

	config := &RsEncoderConfig{
		ParityRatio:      0.5,
		MessageChunkSize: 1024,
		NetworkChunkSize: 1030,
		ElementsPerChunk: 1024 / len(f.One().Bytes()),
		Field:            f,
		PrimitiveElement: primitiveElement,
	}

	encoder1, _ := NewRsEncoder(config)
	encoder2, _ := NewRsEncoder(config)

	// Generate chunks
	message := make([]byte, 16*1024)
	for i := range message {
		message[i] = byte(i % 256)
	}

	messageID := "bench-msg"
	numChunks, _ := encoder1.GenerateThenAddChunks(messageID, message)

	// Collect chunks
	chunks := make([]encode.Chunk, numChunks)
	for i := 0; i < numChunks; i++ {
		chunks[i], _ = encoder1.EmitChunk(peer.ID(""), messageID)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chunk := chunks[i%numChunks]
		encoder2.VerifyThenAddChunk(pubsub.PeerSend{}, chunk)
	}
}

func BenchmarkRsReconstructMessage(b *testing.B) {
	f := field.NewBinaryField(8, big.NewInt(0x11B))
	primitiveElement := f.FromBytes([]byte{0x03})

	config := &RsEncoderConfig{
		ParityRatio:      0.5,
		MessageChunkSize: 1024,
		NetworkChunkSize: 1030,
		ElementsPerChunk: 1024 / len(f.One().Bytes()),
		Field:            f,
		PrimitiveElement: primitiveElement,
	}

	encoder, _ := NewRsEncoder(config)
	decoder, _ := NewRsEncoder(config)

	// Generate test message
	message := make([]byte, 16*1024)
	for i := range message {
		message[i] = byte(i % 256)
	}

	// Pre-generate multiple message IDs and their chunks
	numMessages := 10
	messageIDs := make([]string, numMessages)

	for i := 0; i < numMessages; i++ {
		messageIDs[i] = fmt.Sprintf("msg-%d", i)
		numChunks, _ := encoder.GenerateThenAddChunks(messageIDs[i], message)

		// Add minimum number of chunks to decoder
		for j := 0; j < numChunks; j++ {
			chunk, _ := encoder.EmitChunk(peer.ID(""), messageIDs[i])
			if !decoder.VerifyThenAddChunk(pubsub.PeerSend{}, chunk) {
				break
			}

			// Check if we have enough chunks
			if decoder.GetChunkCount(messageIDs[i]) >= decoder.GetMinChunksForReconstruction(messageIDs[i]) {
				break
			}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		messageID := messageIDs[i%numMessages]
		_, err := decoder.ReconstructMessage(messageID)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRsEndToEnd(b *testing.B) {
	// Benchmark complete encoding/decoding cycle
	f := field.NewBinaryField(8, big.NewInt(0x11B))
	primitiveElement := f.FromBytes([]byte{0x03})

	config := &RsEncoderConfig{
		ParityRatio:      0.5,
		MessageChunkSize: 1024,
		NetworkChunkSize: 1030,
		ElementsPerChunk: 1024 / len(f.One().Bytes()),
		Field:            f,
		PrimitiveElement: primitiveElement,
	}

	encoder, _ := NewRsEncoder(config)
	decoder, _ := NewRsEncoder(config)

	// Test message
	message := make([]byte, 16*1024)
	for i := range message {
		message[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		messageID := fmt.Sprintf("msg-%d", i)
		numChunks, err := encoder.GenerateThenAddChunks(messageID, message)
		if err != nil {
			b.Fatal(err)
		}

		// Decoder receives minimum chunks
		for j := 0; j < numChunks; j++ {
			chunk, _ := encoder.EmitChunk(peer.ID(""), messageID)
			decoder.VerifyThenAddChunk(pubsub.PeerSend{}, chunk)

			if decoder.GetChunkCount(messageID) >= decoder.GetMinChunksForReconstruction(messageID) {
				break
			}
		}

		_, err = decoder.ReconstructMessage(messageID)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestBitmapCompletionSignal(t *testing.T) {
	// Create Reed-Solomon encoder with GF(2^8)
	irreducible := big.NewInt(0x11B)
	f := field.NewBinaryField(8, irreducible)

	config := &RsEncoderConfig{
		ParityRatio:      0.5,
		MessageChunkSize: 32,
		NetworkChunkSize: 32,
		ElementsPerChunk: 32,
		Field:            f,
		PrimitiveElement: f.FromBytes([]byte{0x03}),
		BitmapThreshold:  0.5, // Start sending bitmap at 50%
	}

	encoder, err := NewRsEncoder(config)
	if err != nil {
		t.Fatalf("Failed to create encoder: %v", err)
	}

	// Create a message
	messageID := "test-bitmap-message"
	message := []byte("Hello, this is a test message!  ")

	numChunks, err := encoder.GenerateThenAddChunks(messageID, message)
	if err != nil {
		t.Fatalf("Failed to generate chunks: %v", err)
	}

	// Create two peer IDs
	peerAlice := peer.ID("alice")
	peerBob := peer.ID("bob")

	t.Run("emit to peer without bitmap", func(t *testing.T) {
		// Should succeed - Alice hasn't sent bitmap yet
		chunk, err := encoder.EmitChunk(peerAlice, messageID)
		if err != nil {
			t.Errorf("Expected success emitting to Alice, got error: %v", err)
		}
		if chunk == nil {
			t.Error("Expected chunk to be returned")
		}
	})

	t.Run("peer sends chunk with bitmap", func(t *testing.T) {
		// Create a chunk with a bitmap from Alice
		chunkWithBitmap := Chunk{
			MessageID:  messageID,
			Index:      0,
			ChunkData:  make([]byte, 32),
			ChunkCount: numChunks,
			Bitmap:     []byte{0xFF}, // Non-empty bitmap signals completion
		}

		// Add the chunk from Alice
		peerAliceSend := pubsub.PeerSend{ID: peerAlice}
		accepted := encoder.VerifyThenAddChunk(peerAliceSend, chunkWithBitmap)
		if !accepted {
			// It's ok if chunk is rejected as duplicate (index 0 may already exist)
			// What matters is that the bitmap was recorded
		}
	})

	t.Run("emit blocked after bitmap received", func(t *testing.T) {
		// Should fail - Alice has sent bitmap
		chunk, err := encoder.EmitChunk(peerAlice, messageID)
		if err == nil {
			t.Errorf("Expected error emitting to Alice after bitmap, but got success with chunk: %v", chunk)
		}
		if err != nil && chunk != nil {
			t.Error("Expected nil chunk when error is returned")
		}
	})

	t.Run("emit to different peer still works", func(t *testing.T) {
		// Should succeed - Bob hasn't sent bitmap
		chunk, err := encoder.EmitChunk(peerBob, messageID)
		if err != nil {
			t.Errorf("Expected success emitting to Bob, got error: %v", err)
		}
		if chunk == nil {
			t.Error("Expected chunk to be returned")
		}
	})

	t.Run("multiple peers tracked independently", func(t *testing.T) {
		// Bob sends bitmap
		chunkWithBitmap := Chunk{
			MessageID:  messageID,
			Index:      1,
			ChunkData:  make([]byte, 32),
			ChunkCount: numChunks,
			Bitmap:     []byte{0xFF},
		}

		peerBobSend := pubsub.PeerSend{ID: peerBob}
		encoder.VerifyThenAddChunk(peerBobSend, chunkWithBitmap)

		// Both Alice and Bob should now be blocked
		_, err1 := encoder.EmitChunk(peerAlice, messageID)
		_, err2 := encoder.EmitChunk(peerBob, messageID)

		if err1 == nil {
			t.Error("Expected error emitting to Alice")
		}
		if err2 == nil {
			t.Error("Expected error emitting to Bob")
		}

		// A new peer should still work
		peerCharlie := peer.ID("charlie")
		chunk, err := encoder.EmitChunk(peerCharlie, messageID)
		if err != nil {
			t.Errorf("Expected success emitting to Charlie, got error: %v", err)
		}
		if chunk == nil {
			t.Error("Expected chunk to be returned")
		}
	})
}

func TestBitmapSendsMissingChunks(t *testing.T) {
	// Create Reed-Solomon encoder with GF(2^8)
	irreducible := big.NewInt(0x11B)
	f := field.NewBinaryField(8, irreducible)

	config := &RsEncoderConfig{
		ParityRatio:      0.5,
		MessageChunkSize: 32,
		NetworkChunkSize: 32,
		ElementsPerChunk: 32,
		Field:            f,
		PrimitiveElement: f.FromBytes([]byte{0x03}),
		BitmapThreshold:  0.5,
	}

	encoder, err := NewRsEncoder(config)
	if err != nil {
		t.Fatalf("Failed to create encoder: %v", err)
	}

	// Create a message - use 128 bytes to get multiple chunks
	messageID := "test-missing-chunks"
	message := make([]byte, 128)
	for i := range message {
		message[i] = byte(i)
	}

	numChunks, err := encoder.GenerateThenAddChunks(messageID, message)
	if err != nil {
		t.Fatalf("Failed to generate chunks: %v", err)
	}

	// With 128 byte message and 32 byte chunks: 4 data chunks + 2 parity chunks = 6 total
	t.Logf("Generated %d data chunks, total with parity: %d", numChunks, numChunks+encoder.getParityCount(numChunks))

	// Track chunks sent to peer
	var sentChunks []*pb.EcRpc_Chunk
	mockSendFunc := func(rpc *pb.TopicRpc) {
		if rpc.GetEc() != nil {
			sentChunks = append(sentChunks, rpc.GetEc().GetChunks()...)
		}
	}

	// Create a bitmap requesting chunks 0 and 2
	// We want chunks at indices 0 and 2
	bitmap := make([]byte, 1)
	bitmap[0] = 0x05 // Binary: 00000101 (bits 0 and 2 set)

	chunkWithBitmap := Chunk{
		MessageID:  messageID,
		Index:      1, // Sending chunk 1 with bitmap
		ChunkData:  make([]byte, 32),
		ChunkCount: numChunks,
		Bitmap:     bitmap,
	}

	peerAlice := pubsub.PeerSend{
		ID:       peer.ID("alice"),
		SendFunc: mockSendFunc,
	}

	// Add the chunk with bitmap
	encoder.VerifyThenAddChunk(peerAlice, chunkWithBitmap)

	// Verify that chunks were sent
	if len(sentChunks) == 0 {
		t.Error("Expected missing chunks to be sent, but none were sent")
	}

	// Verify we received the chunks we requested
	receivedIndices := make(map[int]bool)
	for _, chunk := range sentChunks {
		// Decode the chunk to get its index
		decodedChunk, err := encoder.DecodeChunk(*chunk.MessageID, chunk.Data, chunk.Extra)
		if err != nil {
			t.Errorf("Failed to decode sent chunk: %v", err)
			continue
		}
		rsChunk := decodedChunk.(Chunk)
		receivedIndices[rsChunk.Index] = true
	}

	// Check that we received chunk 0 and chunk 2
	if !receivedIndices[0] {
		t.Error("Expected to receive chunk 0, but didn't")
	}
	if !receivedIndices[2] {
		t.Error("Expected to receive chunk 2, but didn't")
	}

	t.Logf("Successfully sent %d missing chunks as requested by bitmap", len(sentChunks))
}

// TestBitmapPendingChunkRequests tests that chunk requests are tracked and fulfilled later
func TestBitmapPendingChunkRequests(t *testing.T) {
	// Create encoder without generating chunks initially
	irreducible := big.NewInt(0x11B)
	f := field.NewBinaryField(8, irreducible)
	config := &RsEncoderConfig{
		ParityRatio:      0.5,
		MessageChunkSize: 32,
		NetworkChunkSize: 32,
		ElementsPerChunk: 32,
		Field:            f,
		PrimitiveElement: f.FromBytes([]byte{0x03}),
		BitmapThreshold:  0.5,
	}

	encoder, err := NewRsEncoder(config)
	if err != nil {
		t.Fatalf("Failed to create encoder: %v", err)
	}

	messageID := "test-pending-requests"

	// Manually add only chunk 0 to the encoder
	chunk0 := Chunk{
		MessageID:  messageID,
		Index:      0,
		ChunkData:  make([]byte, 32),
		ChunkCount: 4, // 4 data chunks
	}
	for i := range chunk0.ChunkData {
		chunk0.ChunkData[i] = byte(i)
	}

	encoder.mutex.Lock()
	encoder.chunks[messageID] = make(map[int]Chunk)
	encoder.chunks[messageID][0] = chunk0
	encoder.chunkCounts[messageID] = 4
	encoder.chunkOrder[messageID] = []int{0}
	encoder.emitCounts[messageID] = make(map[int]int)
	encoder.mutex.Unlock()

	// Track chunks sent to peer
	var sentChunks []*pb.EcRpc_Chunk
	mockSendFunc := func(rpc *pb.TopicRpc) {
		if rpc.GetEc() != nil {
			sentChunks = append(sentChunks, rpc.GetEc().GetChunks()...)
		}
	}

	peerAlice := pubsub.PeerSend{
		ID:       peer.ID("alice"),
		SendFunc: mockSendFunc,
	}

	// Create a bitmap requesting chunks 0, 1, and 2
	bitmap := make([]byte, 1)
	bitmap[0] = 0x07 // Binary: 00000111 (bits 0, 1, 2 set)

	chunkWithBitmap := Chunk{
		MessageID:  messageID,
		Index:      3, // Sending chunk 3 with bitmap
		ChunkData:  make([]byte, 32),
		ChunkCount: 4,
		Bitmap:     bitmap,
	}

	// Add the chunk with bitmap - should send chunk 0 immediately and track requests for 1 and 2
	if !encoder.VerifyThenAddChunk(peerAlice, chunkWithBitmap) {
		t.Fatal("Failed to add chunk with bitmap")
	}

	// Verify that only chunk 0 was sent immediately
	if len(sentChunks) != 1 {
		t.Fatalf("Expected 1 chunk to be sent immediately, got %d", len(sentChunks))
	}

	decodedChunk, err := encoder.DecodeChunk(*sentChunks[0].MessageID, sentChunks[0].Data, sentChunks[0].Extra)
	if err != nil {
		t.Fatalf("Failed to decode sent chunk: %v", err)
	}
	if decodedChunk.(Chunk).Index != 0 {
		t.Errorf("Expected chunk 0 to be sent immediately, got chunk %d", decodedChunk.(Chunk).Index)
	}

	t.Logf("Chunk 0 sent immediately as expected")

	// Clear sent chunks to track new ones
	sentChunks = nil

	// Now add chunk 1 - should be automatically sent to alice
	chunk1 := Chunk{
		MessageID:  messageID,
		Index:      1,
		ChunkData:  make([]byte, 32),
		ChunkCount: 4,
	}
	for i := range chunk1.ChunkData {
		chunk1.ChunkData[i] = byte(i + 32)
	}

	// Add chunk 1 from a different peer (not alice)
	peerBob := pubsub.PeerSend{
		ID:       peer.ID("bob"),
		SendFunc: nil, // Bob doesn't need a send function
	}

	if !encoder.VerifyThenAddChunk(peerBob, chunk1) {
		t.Fatal("Failed to add chunk 1")
	}

	// Verify that chunk 1 was sent to alice
	if len(sentChunks) != 1 {
		t.Fatalf("Expected 1 chunk to be sent to alice after chunk 1 arrives, got %d", len(sentChunks))
	}

	decodedChunk, err = encoder.DecodeChunk(*sentChunks[0].MessageID, sentChunks[0].Data, sentChunks[0].Extra)
	if err != nil {
		t.Fatalf("Failed to decode sent chunk: %v", err)
	}
	if decodedChunk.(Chunk).Index != 1 {
		t.Errorf("Expected chunk 1 to be sent to alice, got chunk %d", decodedChunk.(Chunk).Index)
	}

	t.Logf("Chunk 1 automatically sent to alice when it arrived")

	// Clear sent chunks again
	sentChunks = nil

	// Now add chunk 2 - should also be automatically sent to alice
	chunk2 := Chunk{
		MessageID:  messageID,
		Index:      2,
		ChunkData:  make([]byte, 32),
		ChunkCount: 4,
	}
	for i := range chunk2.ChunkData {
		chunk2.ChunkData[i] = byte(i + 64)
	}

	if !encoder.VerifyThenAddChunk(peerBob, chunk2) {
		t.Fatal("Failed to add chunk 2")
	}

	// Verify that chunk 2 was sent to alice
	if len(sentChunks) != 1 {
		t.Fatalf("Expected 1 chunk to be sent to alice after chunk 2 arrives, got %d", len(sentChunks))
	}

	decodedChunk, err = encoder.DecodeChunk(*sentChunks[0].MessageID, sentChunks[0].Data, sentChunks[0].Extra)
	if err != nil {
		t.Fatalf("Failed to decode sent chunk: %v", err)
	}
	if decodedChunk.(Chunk).Index != 2 {
		t.Errorf("Expected chunk 2 to be sent to alice, got chunk %d", decodedChunk.(Chunk).Index)
	}

	t.Logf("Chunk 2 automatically sent to alice when it arrived")

	// Verify that alice received all 3 chunks (0 immediately, 1 and 2 later)
	t.Logf("Successfully tracked pending requests and fulfilled them when chunks arrived")
}
