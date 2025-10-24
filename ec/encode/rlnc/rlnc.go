package rlnc

import (
	"fmt"
	"math/big"
	"slices"
	"sync"

	"github.com/ethp2p/eth-ec-broadcast/ec/encode"
	"github.com/ethp2p/eth-ec-broadcast/ec/field"
	"github.com/ethp2p/eth-ec-broadcast/pb"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("rlnc")

// Chunk represents a network coding chunk with its linear combination coefficients
type Chunk struct {
	MessageID string          // The ID of the message this chunk belongs to
	ChunkData []byte          // The actual chunk data (encoded for network transmission)
	Coeffs    []field.Element // Coefficients for the linear combination this chunk represents
	Extra     []byte          // Optional extra data (verification info, signatures, etc.)
}

// Data returns the chunk's data bytes (implements encode.Chunk interface)
func (c Chunk) Data() []byte {
	return c.ChunkData
}

// EncodeExtra serializes the chunk's coefficients and extra data to RlncExtra protobuf
func (c Chunk) EncodeExtra() []byte {
	rlncExtra := &pb.RlncExtra{
		Extra: c.Extra,
	}

	// Convert field elements to bytes
	for _, coeff := range c.Coeffs {
		rlncExtra.Coefficients = append(rlncExtra.Coefficients, coeff.Bytes())
	}

	data, _ := rlncExtra.Marshal()
	return data
}

type RlncEncoderConfig struct {
	// Message chunk size in bytes. When the message is larger than this size, it will be chunked, so
	// every chunk will be of this size.
	MessageChunkSize int
	// Network chunk size in bytes. This is the actual size of chunks sent into the network.
	NetworkChunkSize int
	// The number of field elements per chunk.
	ElementsPerChunk int
	// Max coeficient bits used in linear combinations.
	MaxCoefficientBits int
	// Finite field for linear algebra operations
	Field field.Field
	// Chunk verifier for verification (optional)
	Verifier ChunkVerifier
}

type RlncEncoder struct {
	config *RlncEncoderConfig

	messageBitsPerElement int // Bits per field element for message data
	networkBitsPerElement int // Bits per field element for network data

	mutex     sync.Mutex                   // Protects chunks map and REF data
	chunks    map[string][]Chunk           // Storage for chunks by message ID
	coeffsREF map[string][][]field.Element // REF form of coefficient vectors by message ID
}

func NewRlncEncoder(config *RlncEncoderConfig) (*RlncEncoder, error) {
	if config == nil {
		config = DefaultRlncEncoderConfig()
	}
	r := &RlncEncoder{
		config:    config,
		chunks:    make(map[string][]Chunk),
		coeffsREF: make(map[string][][]field.Element),
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

// VerifyThenAddChunk verifies a chunk and stores it if valid and linearly independent
func (r *RlncEncoder) VerifyThenAddChunk(chunk encode.Chunk) bool {
	// Convert generic chunk to RLNC chunk type
	rlncChunk, ok := chunk.(Chunk)
	if !ok {
		return false
	}

	// Check linear independence against existing chunks for this message
	messageID := rlncChunk.MessageID

	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Use REF-optimized incremental independence check
	existingREF := r.coeffsREF[messageID]
	newREF, isIndependent := field.IsLinearlyIndependentIncremental(existingREF, rlncChunk.Coeffs, r.config.Field)
	if !isIndependent {
		// Linearly dependent chunk
		return false
	}

	// Call internal verifier if available (after linear independence check)
	if r.config.Verifier != nil && !r.config.Verifier.Verify(&rlncChunk) {
		// Verification failed on linearly independent chunk
		return false
	}

	// Add chunk to storage and update REF
	r.chunks[messageID] = append(r.chunks[messageID], rlncChunk)
	r.coeffsREF[messageID] = newREF

	return true
}

func DefaultRlncEncoderConfig() *RlncEncoderConfig {
	// Use a large prime field by default: 2^256 + 297 (prime)
	// This gives us good security properties for linear combinations
	p := new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil)
	p.Add(p, big.NewInt(297))
	finiteField := field.NewPrimeField(p)

	return &RlncEncoderConfig{
		MessageChunkSize:   1024, // 1KB chunks are a good balance
		NetworkChunkSize:   1030, // Slightly larger to accommodate field encoding
		MaxCoefficientBits: 16,   // 16-bit coefficients give good randomness
		ElementsPerChunk:   8 * 1024 / finiteField.BitsPerDataElement(),
		Field:              finiteField,
	}
}

// EmitChunk emits a chunk to be sent out from the stored chunks of a given message ID
func (r *RlncEncoder) EmitChunk(messageID string) (encode.Chunk, error) {
	// Get chunks from internal storage
	r.mutex.Lock()
	if len(r.chunks[messageID]) == 0 {
		r.mutex.Unlock()
		return nil, fmt.Errorf("no chunks found for message ID: %s", messageID)
	}
	// Make a copy to avoid holding the lock during computation
	rlncChunks := make([]Chunk, len(r.chunks[messageID]))
	copy(rlncChunks, r.chunks[messageID])
	r.mutex.Unlock()

	// Create random linear combination
	var randomFactors []field.Element
	accumulator := make([]field.Element, r.config.ElementsPerChunk)
	for i := range accumulator {
		accumulator[i] = r.config.Field.Zero()
	}

	for _, chunk := range rlncChunks {
		fieldElements := field.SplitBitsToFieldElements(chunk.ChunkData, r.networkBitsPerElement, r.config.Field)
		// Pick a random coefficient for this chunk
		randomFactor, err := r.config.Field.RandomMax(r.config.MaxCoefficientBits)
		if err != nil {
			return nil, err
		}
		randomFactors = append(randomFactors, randomFactor)
		for i, element := range fieldElements {
			// accumulator[i] = accumulator[i] + randomFactor * element
			accumulator[i] = accumulator[i].Add(randomFactor.Mul(element))
		}
	}

	// Update the coefficient vectors: new_coeffs[i] = sum(factor[j] * old_coeffs[j][i])
	combinedCoefficients := make([]field.Element, len(rlncChunks[0].Coeffs))
	for i := range combinedCoefficients {
		combinedCoefficients[i] = r.config.Field.Zero()
		for j, randomFactor := range randomFactors {
			// combinedCoefficients[i] = combinedCoefficients[i] + randomFactor * chunks[j].Coeffs[i]
			combinedCoefficients[i] = combinedCoefficients[i].Add(randomFactor.Mul(rlncChunks[j].Coeffs[i]))
		}
	}

	// Let the verifier handle combination of extra data (if any)
	var extraData []byte
	if r.config.Verifier != nil {
		var err error
		extraData, err = r.config.Verifier.CombineExtras(rlncChunks, randomFactors)
		if err != nil {
			return Chunk{}, err
		}
	}

	combinedData := field.FieldElementsToBytes(accumulator, r.networkBitsPerElement)
	combinedChunk := Chunk{
		MessageID: messageID,
		ChunkData: combinedData,
		Coeffs:    combinedCoefficients,
		Extra:     extraData,
	}
	return combinedChunk, nil
}

// GenerateThenAddChunks splits a message into chunks and stores them with identity coefficients
func (r *RlncEncoder) GenerateThenAddChunks(messageID string, message []byte) (int, error) {
	if len(message)%r.config.MessageChunkSize != 0 {
		return 0, fmt.Errorf("the size of the message (%d) must be a multiple of the chunk size (%d)",
			len(message), r.config.MessageChunkSize)
	}

	messageBuffer := message
	var chunks []Chunk

	// Split message into fixed-size chunks and prepare for network encoding
	var chunkBuffers [][]byte
	for len(messageBuffer) > 0 {
		chunkData := messageBuffer[:r.config.MessageChunkSize]

		// Convert to field elements and re-encode for network transmission
		fieldElements := field.SplitBitsToFieldElements(chunkData, r.messageBitsPerElement, r.config.Field)
		chunkData = field.FieldElementsToBytes(fieldElements, r.networkBitsPerElement)

		chunks = append(chunks, Chunk{
			MessageID: messageID,
			ChunkData: chunkData,
		})
		chunkBuffers = append(chunkBuffers, chunkData)
		messageBuffer = messageBuffer[r.config.MessageChunkSize:]
	}

	// Set up identity matrix for coefficients - each chunk initially represents itself
	// This is the foundation that allows reconstruction via linear algebra
	coefficients := make([]field.Element, len(chunks))
	for i := range chunks {
		coefficients[i] = r.config.Field.Zero()
	}
	for i := range chunks {
		coefficients[i] = r.config.Field.One()        // chunk i has coefficient 1 for itself
		chunks[i].Coeffs = slices.Clone(coefficients) // copy the coefficient vector
		coefficients[i] = r.config.Field.Zero()       // reset for next chunk
	}

	// Generate extra fields
	if r.config.Verifier != nil {
		extras, err := r.config.Verifier.GenerateExtras(chunkBuffers)
		if err != nil {
			return 0, err
		}
		for i := range chunks {
			chunks[i].Extra = extras[i]
		}
	}

	// Store chunks in internal storage
	r.mutex.Lock()
	r.chunks[messageID] = chunks

	// Identity matrix is already in REF - just copy coefficient vectors directly
	r.coeffsREF[messageID] = make([][]field.Element, len(chunks))
	for i, chunk := range chunks {
		r.coeffsREF[messageID][i] = make([]field.Element, len(chunk.Coeffs))
		copy(r.coeffsREF[messageID][i], chunk.Coeffs)
	}
	r.mutex.Unlock()

	return len(chunks), nil
}

// GetChunks returns all chunks for a given message ID
func (r *RlncEncoder) GetChunks(messageID string) []Chunk {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	chunks := r.chunks[messageID]
	// Return a copy to avoid external modifications
	result := make([]Chunk, len(chunks))
	copy(result, chunks)
	return result
}

// GetMessageIDs returns all message IDs that have chunks stored
func (r *RlncEncoder) GetMessageIDs() []string {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	var ids []string
	for id := range r.chunks {
		ids = append(ids, id)
	}
	return ids
}

// GetChunkCount returns the number of chunks for a message ID
func (r *RlncEncoder) GetChunkCount(messageID string) int {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return len(r.chunks[messageID])
}

// GetMinChunksForReconstruction returns the minimum number of chunks needed to reconstruct a message
func (r *RlncEncoder) GetMinChunksForReconstruction(messageID string) int {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	chunks := r.chunks[messageID]
	if len(chunks) == 0 {
		return 0
	}
	// The minimum number of chunks needed is equal to the number of coefficients
	// (i.e., the number of original chunks that were encoded)
	return len(chunks[0].Coeffs)
}

// GetChunksBeforeCompletion returns the number of chunks needed before sending completion signal
func (r *RlncEncoder) GetChunksBeforeCompletion(messageID string) int {
	// For RLNC, we send completion signal as soon as we can reconstruct,
	// since there's no fixed total number of chunks
	return r.GetMinChunksForReconstruction(messageID)
}

// ReconstructMessage recovers the original message by solving the linear system
func (r *RlncEncoder) ReconstructMessage(messageID string) ([]byte, error) {
	/*
	   To recover the original vectors v₁, ..., vₙ from n random linear combinations r₁, ..., rₙ over a field, we assume
	   the random linear combinations are linearly independent, i.e., the matrix of coefficients used to produce the combinations
	   is invertible. So:

	   Let V be an n × m matrix where each row is a vector vᵢ.

	   Let A be an n × n matrix of random coefficients.

	   Let R = A · V, where each row of R is rᵢ.

	   To recover V, compute A⁻¹ · R.
	*/

	// Get chunks from internal storage
	r.mutex.Lock()
	if len(r.chunks[messageID]) == 0 {
		r.mutex.Unlock()
		return nil, fmt.Errorf("no chunks found for message ID: %s", messageID)
	}
	// Make a copy to avoid holding the lock during computation
	rlncChunks := make([]Chunk, len(r.chunks[messageID]))
	copy(rlncChunks, r.chunks[messageID])
	r.mutex.Unlock()

	// Build coefficient matrix A and data matrix R
	var A [][]field.Element
	var R [][]field.Element

	for _, chunk := range rlncChunks {
		r := field.SplitBitsToFieldElements(chunk.ChunkData, r.networkBitsPerElement, r.config.Field)
		A = append(A, chunk.Coeffs)
		R = append(R, r)
	}

	// Solve A * V = R to recover original vectors V
	V, err := field.RecoverVectors(A, R, r.config.Field)
	if err != nil {
		return nil, err
	}

	// Convert field elements back to message bytes
	var messageBuffer []byte
	for _, v := range V {
		chunkData := field.FieldElementsToBytes(v, r.messageBitsPerElement)
		messageBuffer = append(messageBuffer, chunkData...)
	}

	return messageBuffer, nil
}

// DecodeChunk deserializes RlncExtra protobuf data and creates a chunk with given messageID and data
func (r *RlncEncoder) DecodeChunk(messageID string, data []byte, extra []byte) (encode.Chunk, error) {
	rlncExtra := &pb.RlncExtra{}
	if err := rlncExtra.Unmarshal(extra); err != nil {
		return nil, err
	}

	var coeffs []field.Element
	for _, coeffBytes := range rlncExtra.Coefficients {
		coeffs = append(coeffs, r.config.Field.FromBytes(coeffBytes))
	}

	return Chunk{
		MessageID: messageID,
		ChunkData: data,
		Coeffs:    coeffs,
		Extra:     rlncExtra.Extra,
	}, nil
}
