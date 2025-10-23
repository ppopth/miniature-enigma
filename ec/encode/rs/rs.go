package rs

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/ethp2p/eth-ec-broadcast/ec/encode"
	"github.com/ethp2p/eth-ec-broadcast/ec/field"
	"github.com/ethp2p/eth-ec-broadcast/pb"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/peer"
)

var log = logging.Logger("rs")

// Chunk represents a Reed-Solomon encoded chunk
type Chunk struct {
	MessageID  string // The ID of the message this chunk belongs to
	Index      int    // The index of this chunk (0 to k+n-1)
	ChunkData  []byte // The actual chunk data
	ChunkCount int    // Number of data chunks for this message
	Extra      []byte // Optional extra data (verification info, signatures, etc.)
	Bitmap     []byte // Bitmap of missing chunks (bit i = 1 means chunk i is missing)
}

// Data returns the chunk's data bytes (implements encode.Chunk interface)
func (c Chunk) Data() []byte {
	return c.ChunkData
}

// EncodeExtra serializes the chunk's index, bitmap, and extra data to RsExtra protobuf
func (c Chunk) EncodeExtra() []byte {
	index := uint32(c.Index)
	chunkCount := uint32(c.ChunkCount)
	rsExtra := &pb.RsExtra{
		Index:      &index,
		ChunkCount: &chunkCount,
		Extra:      c.Extra,
	}

	// Include bitmap if present
	if len(c.Bitmap) > 0 {
		rsExtra.IwantBitmap = c.Bitmap
	}

	data, _ := rsExtra.Marshal()
	return data
}

// RsEncoderConfig contains configuration for Reed-Solomon encoder
type RsEncoderConfig struct {
	// Parity ratio (e.g., 0.5 means 50% redundancy, 2.0 means 200% redundancy)
	ParityRatio float64
	// Message chunk size in bytes
	MessageChunkSize int
	// Network chunk size in bytes
	NetworkChunkSize int
	// The number of field elements per chunk
	ElementsPerChunk int
	// Finite field for operations
	Field field.Field
	// Primitive element for generating evaluation points
	// Must be provided when Field is set
	PrimitiveElement field.Element
	// Optional chunk verifier for validating chunks
	ChunkVerifier ChunkVerifier
	// Threshold for starting to send bitmap (e.g., 0.3 = start sending when 70% of chunks received)
	// Bitmap starts when receivedCount >= (1 - BitmapThreshold) * totalChunks
	BitmapThreshold float64
}

// RsEncoder implements Reed-Solomon erasure coding
type RsEncoder struct {
	config *RsEncoderConfig

	messageBitsPerElement int // Bits per field element for message data
	networkBitsPerElement int // Bits per field element for network data

	mutex       sync.Mutex               // Protects chunks map
	chunks      map[string]map[int]Chunk // Storage for chunks by message ID and index
	chunkCounts map[string]int           // Number of data chunks per message
	chunkOrder  map[string][]int         // Order in which chunks were received
	emitCounts  map[string]map[int]int   // Track how many times each chunk has been emitted

	// Track which peers have sent bitmaps for each message
	// Map structure: messageID -> set of peer IDs who sent bitmaps
	peerBitmaps map[string]map[peer.ID]struct{}
}

// NewRsEncoder creates a new Reed-Solomon encoder
func NewRsEncoder(config *RsEncoderConfig) (*RsEncoder, error) {
	if config == nil {
		config = DefaultRsEncoderConfig()
	}

	if config.ParityRatio <= 0 {
		return nil, fmt.Errorf("parity ratio must be positive")
	}
	if config.MessageChunkSize <= 0 {
		return nil, fmt.Errorf("message chunk size must be positive")
	}
	if config.NetworkChunkSize <= 0 {
		return nil, fmt.Errorf("network chunk size must be positive")
	}
	if config.ElementsPerChunk <= 0 {
		return nil, fmt.Errorf("elements per chunk must be positive")
	}
	if config.Field != nil && config.PrimitiveElement == nil {
		return nil, fmt.Errorf("primitive element must be provided when field is set")
	}

	r := &RsEncoder{
		config:      config,
		chunks:      make(map[string]map[int]Chunk),
		chunkCounts: make(map[string]int),
		chunkOrder:  make(map[string][]int),
		emitCounts:  make(map[string]map[int]int),
		peerBitmaps: make(map[string]map[peer.ID]struct{}),
	}

	if (8*r.config.MessageChunkSize)%r.config.ElementsPerChunk != 0 {
		return nil, fmt.Errorf("ElementsPerChunk (%d) must divide 8*MessageChunkSize (%d)",
			r.config.ElementsPerChunk, 8*r.config.MessageChunkSize)
	}

	// Calculate and validate bit allocations for field element encoding
	// This is critical - we need to ensure data fits properly in field elements
	r.messageBitsPerElement = (8 * r.config.MessageChunkSize) / r.config.ElementsPerChunk
	if r.messageBitsPerElement > r.config.Field.BitsPerDataElement() {
		return nil, fmt.Errorf("(8*MessageChunkSize)/ElementsPerChunk (%d) is too high for the field", r.messageBitsPerElement)
	}
	r.networkBitsPerElement = (8 * r.config.NetworkChunkSize) / r.config.ElementsPerChunk
	if r.networkBitsPerElement < r.config.Field.BitsPerElement() {
		return nil, fmt.Errorf("(8*NetworkChunkSize)/ElementsPerChunk (%d) is too low for the field", r.networkBitsPerElement)
	}

	return r, nil
}

// DefaultRsEncoderConfig returns default configuration
func DefaultRsEncoderConfig() *RsEncoderConfig {
	// GF(2^8) with irreducible polynomial x^8 + x^4 + x^3 + x + 1
	irreducible := big.NewInt(0x11B)
	f := field.NewBinaryField(8, irreducible)
	return &RsEncoderConfig{
		ParityRatio:      0.5,                       // 50% redundancy
		MessageChunkSize: 1024,                      // Message chunk size in bytes
		NetworkChunkSize: 1024,                      // Network chunk size in bytes (same as message for GF(2^8))
		ElementsPerChunk: 1024,                      // Number of field elements per chunk
		Field:            f,                         // GF(2^8)
		PrimitiveElement: f.FromBytes([]byte{0x03}), // 0x03 is primitive in GF(2^8) with polynomial 0x11B
		BitmapThreshold:  0.5,                       // Start sending bitmap when (1-0.5)=50% of chunks received
	}
}

// VerifyThenAddChunk verifies and stores a chunk if valid
func (r *RsEncoder) VerifyThenAddChunk(peerID peer.ID, chunk encode.Chunk) bool {
	// Convert generic chunk to RS chunk type
	rsChunk, ok := chunk.(Chunk)
	if !ok {
		return false
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check if this chunk contains a bitmap
	if len(rsChunk.Bitmap) > 0 {
		// Initialize the bitmap tracking for this message if needed
		if r.peerBitmaps[rsChunk.MessageID] == nil {
			r.peerBitmaps[rsChunk.MessageID] = make(map[peer.ID]struct{})
		}
		// Mark that this peer has sent a bitmap for this message
		r.peerBitmaps[rsChunk.MessageID][peerID] = struct{}{}
	}

	// Initialize chunk storage for this message if needed
	if _, exists := r.chunks[rsChunk.MessageID]; !exists {
		r.chunks[rsChunk.MessageID] = make(map[int]Chunk)
		r.emitCounts[rsChunk.MessageID] = make(map[int]int)
	}

	// Use the chunk count from the chunk itself
	chunkCount := rsChunk.ChunkCount
	if chunkCount <= 0 {
		// Invalid chunk count
		return false
	}

	// Store/update the chunk count for this message if we don't have it or if it's consistent
	if existingCount, exists := r.chunkCounts[rsChunk.MessageID]; exists {
		if existingCount != chunkCount {
			// Inconsistent chunk count - reject the chunk
			return false
		}
	} else {
		// Store the chunk count for this message
		r.chunkCounts[rsChunk.MessageID] = chunkCount
	}

	// Calculate parity chunks for this message
	parityCount := r.getParityCount(chunkCount)

	// Check if chunk index is valid
	totalChunks := chunkCount + parityCount
	if rsChunk.Index < 0 || rsChunk.Index >= totalChunks {
		return false
	}

	// Check if chunk with same index already exists
	if _, exists := r.chunks[rsChunk.MessageID][rsChunk.Index]; exists {
		return false
	}

	// Use chunk verifier if configured
	if r.config.ChunkVerifier != nil {
		if !r.config.ChunkVerifier.Verify(&rsChunk) {
			return false
		}
	}

	// Store the chunk
	r.chunks[rsChunk.MessageID][rsChunk.Index] = rsChunk
	// Track the order this chunk was received
	r.chunkOrder[rsChunk.MessageID] = append(r.chunkOrder[rsChunk.MessageID], rsChunk.Index)

	return true
}

// getParityCount returns the number of parity chunks for a given data chunk count
func (r *RsEncoder) getParityCount(chunkCount int) int {
	parityCount := int(float64(chunkCount) * r.config.ParityRatio)
	if parityCount == 0 {
		parityCount = 1 // At least 1 parity chunk
	}
	return parityCount
}

// generateBitmap creates a bitmap of missing chunks for a message
// Returns nil if the bitmap threshold hasn't been met or if no chunks are missing
// Bitmap format: bit i = 1 means chunk i is missing (needed)
func (r *RsEncoder) generateBitmap(messageID string) []byte {
	// Must be called while holding r.mutex

	chunkCount, exists := r.chunkCounts[messageID]
	if !exists || chunkCount == 0 {
		return nil
	}

	// Calculate total chunks (data + parity)
	parityCount := r.getParityCount(chunkCount)
	totalChunks := chunkCount + parityCount

	// Check if we've received enough chunks to start sending bitmap
	// Start when receivedCount >= (1 - BitmapThreshold) * totalChunks
	receivedCount := len(r.chunks[messageID])
	threshold := int(float64(totalChunks) * (1.0 - r.config.BitmapThreshold))
	if receivedCount < threshold {
		return nil
	}

	// Check if we already have enough chunks for reconstruction
	if receivedCount >= chunkCount {
		return nil // No need for more chunks
	}

	// Create bitmap: byte array where bit i indicates if chunk i is missing
	bitmapSize := (totalChunks + 7) / 8 // Round up to nearest byte
	bitmap := make([]byte, bitmapSize)

	// Set bits for missing chunks
	for i := 0; i < totalChunks; i++ {
		if _, have := r.chunks[messageID][i]; !have {
			byteIndex := i / 8
			bitIndex := i % 8
			bitmap[byteIndex] |= (1 << bitIndex)
		}
	}

	return bitmap
}

// EmitChunk emits the earliest chunk with the lowest emit count
func (r *RsEncoder) EmitChunk(targetPeerID peer.ID, messageID string) (encode.Chunk, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check if this peer has sent a bitmap for this message
	if peerSet, exists := r.peerBitmaps[messageID]; exists {
		if _, hasBitmap := peerSet[targetPeerID]; hasBitmap {
			return nil, fmt.Errorf("peer %s has sent bitmap for message %s", targetPeerID, messageID)
		}
	}

	chunks, exists := r.chunks[messageID]
	if !exists || len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks available for message %s", messageID)
	}

	// Get the order in which chunks were received
	order, exists := r.chunkOrder[messageID]
	if !exists || len(order) == 0 {
		return nil, fmt.Errorf("no chunk order found for message %s", messageID)
	}

	// Initialize emit counts for this message if needed
	if _, exists := r.emitCounts[messageID]; !exists {
		r.emitCounts[messageID] = make(map[int]int)
	}

	// Generate bitmap of missing chunks (if threshold is met)
	bitmap := r.generateBitmap(messageID)

	// Find the earliest chunk with the lowest emit count
	// Strategy: iterate through all chunks in order, track the one with lowest emit count
	selectedIndex := -1
	lowestEmitCount := -1

	for i := 0; i < len(order); i++ {
		chunkIndex := order[i]
		emitCount := r.emitCounts[messageID][chunkIndex]

		// If this is the first chunk or has lower emit count, select it
		if selectedIndex == -1 || emitCount < lowestEmitCount {
			selectedIndex = i
			lowestEmitCount = emitCount
		}
		// If emit counts are equal, the earlier one (lower i) is already selected
	}

	// Emit the selected chunk
	if selectedIndex != -1 {
		chunkIndex := order[selectedIndex]
		r.emitCounts[messageID][chunkIndex]++
		// Copy chunk and attach bitmap
		chunk := chunks[chunkIndex]
		chunk.Bitmap = bitmap
		return chunk, nil
	}

	return nil, fmt.Errorf("no chunks found for message %s", messageID)
}

// GenerateThenAddChunks splits a message into chunks and generates parity chunks
func (r *RsEncoder) GenerateThenAddChunks(messageID string, message []byte) (int, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check if message size is a multiple of message chunk size
	if len(message)%r.config.MessageChunkSize != 0 {
		return 0, fmt.Errorf("message size (%d) must be a multiple of message chunk size (%d)",
			len(message), r.config.MessageChunkSize)
	}

	// Calculate number of data chunks based on message size
	chunkCount := len(message) / r.config.MessageChunkSize
	if chunkCount == 0 {
		return 0, fmt.Errorf("message is empty")
	}

	// Store the number of data chunks for this message
	r.chunkCounts[messageID] = chunkCount

	// Calculate parity chunks based on ratio
	parityCount := r.getParityCount(chunkCount)

	// Initialize chunk storage
	r.chunks[messageID] = make(map[int]Chunk)
	r.emitCounts[messageID] = make(map[int]int)

	// Store data chunk polynomials for verification if needed
	var dataPolynomials [][]field.Element
	if r.config.ChunkVerifier != nil {
		dataPolynomials = make([][]field.Element, chunkCount)
	}

	// Create data chunks
	for i := 0; i < chunkCount; i++ {
		start := i * r.config.MessageChunkSize
		end := start + r.config.MessageChunkSize
		chunkData := message[start:end]

		// Convert to field elements and re-encode for network transmission
		fieldElements := field.SplitBitsToFieldElements(chunkData, r.messageBitsPerElement, r.config.Field)
		chunkData = field.FieldElementsToBytes(fieldElements, r.networkBitsPerElement)

		// Store polynomial for verification if needed
		if r.config.ChunkVerifier != nil {
			dataPolynomials[i] = fieldElements
		}

		chunk := Chunk{
			MessageID:  messageID,
			Index:      i,
			ChunkData:  chunkData,
			ChunkCount: chunkCount,
		}
		r.chunks[messageID][i] = chunk
		r.chunkOrder[messageID] = append(r.chunkOrder[messageID], i)
	}

	// Generate encoding matrix for this specific message
	// The full matrix has dimensions (chunkCount + parityCount) × chunkCount
	// First chunkCount rows form identity matrix, last parityCount rows form parity matrix
	encodingMatrix := r.generateEncodingMatrix(chunkCount, parityCount)

	// Generate parity chunks using the encoding matrix
	// Mathematical explanation of systematic Reed-Solomon encoding:
	//
	// In systematic encoding, the codeword contains the original data followed by parity.
	// The complete codeword is: [D[0], D[1], ..., D[k-1], P[0], P[1], ..., P[n-k-1]]
	//
	// The systematic generator matrix G is computed as follows:
	// 1. Start with Vandermonde matrix A (size n×k) with evaluation points 1,2,...,n
	// 2. Extract top k×k submatrix A_top
	// 3. Compute A_top^(-1)
	// 4. The systematic generator matrix is G = A × A_top^(-1)
	//    This produces a matrix where:
	//    - First k rows form the identity matrix I (for data chunks)
	//    - Last n-k rows form the parity matrix P (for parity chunks)
	//
	// For each parity chunk P[i], we compute:
	//   P[i] = Σ(j=0 to k-1) encodingMatrix[k+i][j] * D[j]
	//
	// Where:
	//   - k is the number of data chunks
	//   - n-k is the number of parity chunks
	//   - D[j] is the j-th data chunk (as a vector of field elements)
	//   - encodingMatrix[k+i][j] is element from row k+i of the full generator matrix G
	//   - All operations are in the finite field (GF(2^8) by default)
	//
	// Example with 2 data chunks (k=2) and 1 parity chunk (n-k=1):
	//   Original Vandermonde: A = [[1,1], [1,2], [1,3]]
	//   A_top = [[1,1], [1,2]], A_top^(-1) = [[2,1], [1,1]] (in GF(2^8))
	//   Systematic matrix G = A × A_top^(-1) = [[1,0], [0,1], [p0,p1]]
	//   Row 2 (index k=2) contains parity coefficients: [p0,p1]
	//   Therefore: P[0] = p0 * D[0] + p1 * D[1]
	//
	// The accumulator stores the running sum for each field element position.
	// Since each chunk contains ElementsPerChunk field elements, we compute
	// the linear combination element-wise:
	//   P[i][e] = Σ(j=0 to chunkCount-1) encodingMatrix[k+i][j] * D[j][e]
	for i := 0; i < parityCount; i++ {
		// Initialize accumulator for this parity chunk
		accumulator := make([]field.Element, r.config.ElementsPerChunk)
		for k := range accumulator {
			accumulator[k] = r.config.Field.Zero()
		}

		// For each data chunk, multiply by encoding matrix coefficient and add to accumulator
		for j := 0; j < chunkCount; j++ {
			// Extract field elements from this data chunk
			fieldElements := field.SplitBitsToFieldElements(
				r.chunks[messageID][j].ChunkData,
				r.networkBitsPerElement,
				r.config.Field,
			)

			// Multiply each element by the matrix coefficient and add to accumulator
			// This implements: accumulator[k] += encodingMatrix[chunkCount+i][j] * fieldElements[k]
			// We use chunkCount+i because parity rows start after the identity matrix rows
			for k := 0; k < r.config.ElementsPerChunk && k < len(fieldElements); k++ {
				accumulator[k] = accumulator[k].Add(encodingMatrix[chunkCount+i][j].Mul(fieldElements[k]))
			}
		}

		// Convert accumulator back to bytes for network transmission
		parityData := field.FieldElementsToBytes(accumulator, r.networkBitsPerElement)

		chunk := Chunk{
			MessageID:  messageID,
			Index:      chunkCount + i,
			ChunkData:  parityData,
			ChunkCount: chunkCount,
		}

		r.chunks[messageID][chunkCount+i] = chunk
		r.chunkOrder[messageID] = append(r.chunkOrder[messageID], chunkCount+i)
	}

	// Generate verification data for all chunks if verifier is configured
	if r.config.ChunkVerifier != nil {
		totalChunks := chunkCount + parityCount
		evalPoints := r.generateEvaluationPoints(totalChunks)

		// Generate extra data for all chunks (data and parity)
		for i := 0; i < totalChunks; i++ {
			extraData, err := r.config.ChunkVerifier.GenerateExtra(dataPolynomials, evalPoints[i])
			if err != nil {
				return 0, fmt.Errorf("failed to generate verification data for chunk %d: %v", i, err)
			}

			// Update the chunk with extra data
			chunk := r.chunks[messageID][i]
			chunk.Extra = extraData
			r.chunks[messageID][i] = chunk
		}
	}

	return chunkCount + parityCount, nil
}

// ReconstructMessage recovers the original message from available chunks
func (r *RsEncoder) ReconstructMessage(messageID string) ([]byte, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	chunks, exists := r.chunks[messageID]
	if !exists {
		return nil, fmt.Errorf("no chunks found for message %s", messageID)
	}

	// Get the number of data chunks for this message
	chunkCount, exists := r.chunkCounts[messageID]
	if !exists {
		return nil, fmt.Errorf("no data chunk count found for message %s", messageID)
	}

	// Check if we have enough chunks
	if len(chunks) < chunkCount {
		return nil, fmt.Errorf("insufficient chunks: have %d, need %d", len(chunks), chunkCount)
	}

	// Try to reconstruct using only data chunks if available
	hasAllDataChunks := true
	for i := 0; i < chunkCount; i++ {
		if _, exists := chunks[i]; !exists {
			hasAllDataChunks = false
			break
		}
	}

	if hasAllDataChunks {
		// Simple reconstruction from data chunks
		reconstructed := make([]byte, chunkCount*r.config.MessageChunkSize)
		for i := 0; i < chunkCount; i++ {
			// Convert network data back to message data
			fieldElements := field.SplitBitsToFieldElements(chunks[i].ChunkData, r.networkBitsPerElement, r.config.Field)
			messageData := field.FieldElementsToBytes(fieldElements, r.messageBitsPerElement)
			copy(reconstructed[i*r.config.MessageChunkSize:], messageData)
		}
		return reconstructed, nil
	}

	// Complex reconstruction using Reed-Solomon decoding
	//
	// Mathematical explanation of Reed-Solomon erasure decoding:
	//
	// We have a systematic code where the original message is encoded as:
	// C = [D₀, D₁, ..., D_{k-1}, P₀, P₁, ..., P_{n-k-1}]
	// where D_i are data chunks and P_i are parity chunks.
	//
	// The encoding was done using a systematic generator matrix G = [I | P] where:
	// - I is the k×k identity matrix (for data chunks)
	// - P is the k×(n-k) parity matrix
	//
	// Each chunk (data or parity) can be expressed as a linear combination:
	// - Data chunk i: C_i = D_i (trivially)
	// - Parity chunk j: C_{k+j} = Σ(i=0 to k-1) P[j][i] * D_i
	//
	// For decoding with erasures (missing chunks):
	// 1. We need any k chunks out of n total chunks
	// 2. Let's say we have chunks at indices [i₁, i₂, ..., i_k]
	// 3. We form a system of linear equations: A * D = R
	//    where:
	//    - A is a k×k matrix where row j corresponds to the encoding
	//      coefficients for chunk at index i_j
	//    - D = [D₀, D₁, ..., D_{k-1}]ᵀ is the original data vector
	//    - R = [R₁, R₂, ..., R_k]ᵀ is the vector of received chunks
	//
	// 4. To recover D, we solve: D = A⁻¹ * R
	//
	// Example with k=3, n=5, missing chunks at indices 1 and 4:
	// Available chunks: [0, 2, 3] (data chunks 0,2 and parity chunk 0)
	// The decoding matrix A would be:
	// Row 0: [1, 0, 0] (chunk 0 is D₀)
	// Row 1: [0, 0, 1] (chunk 2 is D₂)
	// Row 2: [p₀₀, p₀₁, p₀₂] (chunk 3 is P₀ = p₀₀*D₀ + p₀₁*D₁ + p₀₂*D₂)
	//
	// Collect available chunks and their indices
	availableIndices := make([]int, 0, len(chunks))
	availableChunks := make([]Chunk, 0, len(chunks))

	for idx, chunk := range chunks {
		availableIndices = append(availableIndices, idx)
		availableChunks = append(availableChunks, chunk)
		if len(availableIndices) == chunkCount {
			break
		}
	}

	// Create decoding matrix from the encoding matrix
	// Calculate parity chunks for regenerating matrix
	parityCount := r.getParityCount(chunkCount)
	encodingMatrix := r.generateEncodingMatrix(chunkCount, parityCount)

	// Step 1: Build the decoding matrix A
	// For each available chunk, we extract the corresponding row from the encoding matrix.
	// The encoding matrix has all rows for all chunk positions:
	// - Rows 0 to k-1: Identity matrix (for data chunks)
	// - Rows k to n-1: Parity matrix (for parity chunks)
	//
	// The decoding matrix represents the system of equations:
	// A[i][j] represents the coefficient of D_j in the equation for available chunk i
	decodingMatrix := make([][]field.Element, chunkCount)
	for i := 0; i < chunkCount; i++ {
		decodingMatrix[i] = make([]field.Element, chunkCount)
		idx := availableIndices[i]
		// Simply copy the row from the encoding matrix corresponding to this chunk index
		for j := 0; j < chunkCount; j++ {
			decodingMatrix[i][j] = encodingMatrix[idx][j]
		}
	}

	// Step 2: Invert the decoding matrix
	// We need A⁻¹ to solve for D in the equation A * D = R
	// The invertibility is guaranteed by the MDS (Maximum Distance Separable)
	// property of Reed-Solomon codes: any k×k submatrix is invertible
	invMatrix, err := field.InvertMatrix(decodingMatrix, r.config.Field)
	if err != nil {
		return nil, fmt.Errorf("failed to invert decoding matrix: %v", err)
	}

	// Step 3: Prepare received chunks as field element vectors
	// Each chunk is treated as a vector of field elements
	// R[i] represents the i-th received chunk as a vector
	R := make([][]field.Element, chunkCount)
	for i := 0; i < chunkCount; i++ {
		R[i] = field.SplitBitsToFieldElements(availableChunks[i].ChunkData, r.networkBitsPerElement, r.config.Field)
	}

	// Step 4: Solve D = A⁻¹ * R
	// This recovers the original data chunks D₀, D₁, ..., D_{k-1}
	// We use matrix multiplication: V = invMatrix × R
	// where:
	// - invMatrix is k×k
	// - R is k×ElementsPerChunk (each row is a chunk's field elements)
	// - V is k×ElementsPerChunk (recovered data chunks)
	V := field.MatrixMultiply(invMatrix, R, r.config.Field)

	// Step 5: Convert recovered field elements back to message bytes
	// V now contains the recovered data chunks D₀, D₁, ..., D_{k-1}
	// We concatenate them to form the original message
	reconstructed := make([]byte, chunkCount*r.config.MessageChunkSize)
	for i := 0; i < chunkCount; i++ {
		messageData := field.FieldElementsToBytes(V[i], r.messageBitsPerElement)
		copy(reconstructed[i*r.config.MessageChunkSize:], messageData)
	}

	return reconstructed, nil
}

// DecodeChunk creates a chunk from network data
func (r *RsEncoder) DecodeChunk(messageID string, data []byte, extra []byte) (encode.Chunk, error) {
	// Decode the extra data to get chunk index
	rsExtra := &pb.RsExtra{}
	if err := rsExtra.Unmarshal(extra); err != nil {
		return nil, fmt.Errorf("failed to unmarshal extra data: %v", err)
	}

	// Check if index is nil
	if rsExtra.Index == nil {
		return nil, fmt.Errorf("chunk index is nil")
	}
	index := int(*rsExtra.Index)

	// Check if chunkCount is nil
	if rsExtra.ChunkCount == nil {
		return nil, fmt.Errorf("chunk count is nil")
	}
	chunkCount := int(*rsExtra.ChunkCount)

	return Chunk{
		MessageID:  messageID,
		Index:      index,
		ChunkData:  data,
		ChunkCount: chunkCount,
		Extra:      rsExtra.Extra,
		Bitmap:     rsExtra.IwantBitmap,
	}, nil
}

// GetMessageIDs returns all message IDs that have chunks stored
func (r *RsEncoder) GetMessageIDs() []string {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	ids := make([]string, 0, len(r.chunks))
	for id := range r.chunks {
		ids = append(ids, id)
	}
	return ids
}

// GetChunkCount returns the number of chunks for a message ID
func (r *RsEncoder) GetChunkCount(messageID string) int {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if chunks, exists := r.chunks[messageID]; exists {
		return len(chunks)
	}
	return 0
}

// GetMinChunksForReconstruction returns the minimum number of chunks needed
func (r *RsEncoder) GetMinChunksForReconstruction(messageID string) int {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	chunkCount, exists := r.chunkCounts[messageID]
	if !exists {
		return 0 // Unknown message
	}
	return chunkCount
}

// GetChunksBeforeCompletion returns the total number of chunks (data + parity) before sending completion signal
func (r *RsEncoder) GetChunksBeforeCompletion(messageID string) int {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	chunkCount, exists := r.chunkCounts[messageID]
	if !exists {
		return 0 // Unknown message
	}

	// Calculate parity chunks
	parityCount := int(float64(chunkCount) * r.config.ParityRatio)
	if parityCount == 0 {
		parityCount = 1 // At least 1 parity chunk
	}

	// Return total chunks (data + parity)
	return chunkCount + parityCount
}

// generateEncodingMatrix creates the full systematic generator matrix G = [I | P]
//
// Following the systematic Reed-Solomon encoding procedure from Wikipedia:
// 1. Create a Vandermonde matrix A of size n×k (where n = k + parity chunks)
// 2. Take the first k rows of A (the data portion)
// 3. Compute the inverse of this k×k submatrix
// 4. Multiply the inverse by the full Vandermonde matrix to get G = [I | P]
// 5. Return the full generator matrix where:
//   - The first k rows form the identity matrix I (for data chunks)
//   - The last n-k rows form the parity matrix P (for parity chunks)
func (r *RsEncoder) generateEncodingMatrix(chunkCount int, parityCount int) [][]field.Element {
	totalChunks := chunkCount + parityCount

	// Step 1: Create the full Vandermonde matrix A (n×k)
	// Use powers of the primitive element as evaluation points: 1, α, α^2, ..., α^(n-1)
	// This guarantees the MDS property for Reed-Solomon codes
	vandermonde := make([][]field.Element, totalChunks)
	evalPoints := r.generateEvaluationPoints(totalChunks)

	// Build Vandermonde matrix where V[i][j] = (α^i)^j
	for i := 0; i < totalChunks; i++ {
		vandermonde[i] = make([]field.Element, chunkCount)
		power := r.config.Field.One()
		for j := 0; j < chunkCount; j++ {
			vandermonde[i][j] = power.Clone()
			if j < chunkCount-1 {
				power = power.Mul(evalPoints[i])
			}
		}
	}

	// Step 2: Extract the top k×k submatrix (data portion)
	dataMatrix := make([][]field.Element, chunkCount)
	for i := 0; i < chunkCount; i++ {
		dataMatrix[i] = make([]field.Element, chunkCount)
		for j := 0; j < chunkCount; j++ {
			dataMatrix[i][j] = vandermonde[i][j]
		}
	}

	// Step 3: Compute the inverse of the data matrix
	invDataMatrix, err := field.InvertMatrix(dataMatrix, r.config.Field)
	if err != nil {
		// This should not happen with properly chosen evaluation points
		panic(fmt.Sprintf("failed to invert data matrix: %v", err))
	}

	// Step 4: Multiply Vandermonde by inverse to get systematic generator matrix
	// G = vandermonde × invDataMatrix = [I | P]
	// This produces the systematic generator matrix where:
	// - First k rows are the identity matrix (for data chunks)
	// - Last n-k rows are the parity matrix (for parity chunks)
	generatorMatrix := field.MatrixMultiply(vandermonde, invDataMatrix, r.config.Field)

	return generatorMatrix
}

// generateEvaluationPoints creates evaluation points as powers of the primitive element
func (r *RsEncoder) generateEvaluationPoints(totalChunks int) []field.Element {
	evalPoints := make([]field.Element, totalChunks)

	// Generate evaluation points as powers of primitive element
	evalPoints[0] = r.config.Field.One() // α^0 = 1
	if totalChunks > 1 {
		evalPoints[1] = r.config.PrimitiveElement.Clone() // α^1 = α
		for i := 2; i < totalChunks; i++ {
			evalPoints[i] = evalPoints[i-1].Mul(r.config.PrimitiveElement) // α^i = α^(i-1) * α
		}
	}

	return evalPoints
}
