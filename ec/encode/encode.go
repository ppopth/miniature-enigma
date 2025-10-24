package encode

// Chunk represents a generic encoding chunk
type Chunk interface {
	Data() []byte
	EncodeExtra() []byte
}

// Encoder defines the interface for erasure coding algorithms
type Encoder interface {
	// VerifyThenAddChunk verifies and stores a chunk if valid
	VerifyThenAddChunk(chunk Chunk) bool
	// EmitChunk emits a chunk to be sent out from the stored chunks of a given message ID
	EmitChunk(messageID string) (Chunk, error)
	// GenerateThenAddChunks splits a message into chunks and stores them
	GenerateThenAddChunks(messageID string, message []byte) (int, error)
	// ReconstructMessage recovers the original message
	ReconstructMessage(messageID string) ([]byte, error)
	// DecodeChunk creates a chunk from network data and extra metadata
	DecodeChunk(messageID string, data []byte, extra []byte) (Chunk, error)

	// GetMessageIDs returns all message IDs that have chunks stored
	GetMessageIDs() []string
	// GetChunkCount returns the number of chunks for a message ID
	GetChunkCount(messageID string) int
	// GetMinChunksForReconstruction returns the minimum number of chunks needed to reconstruct a message
	GetMinChunksForReconstruction(messageID string) int
	// GetChunksBeforeCompletion returns the number of chunks needed before sending completion signal
	GetChunksBeforeCompletion(messageID string) int
}
