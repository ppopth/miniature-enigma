package rlnc

import (
	"context"
	"crypto/sha256"
	"fmt"
	"maps"
	"math/big"
	mrand "math/rand"
	"slices"
	"sync"

	"github.com/ethp2p/eth-ec-broadcast/pb"
	"github.com/ethp2p/eth-ec-broadcast/pubsub"
	"github.com/ethp2p/eth-ec-broadcast/rlnc/field"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/peer"
)

var log = logging.Logger("rlnc")

// MsgIdFunc generates unique identifiers for messages
type MsgIdFunc func([]byte) string

// RlncOption configures an RLNC router during construction
type RlncOption func(*RlncRouter) error

// NewRlnc creates a new RLNC router with sensible defaults and applies options
func NewRlnc(opts ...RlncOption) (*RlncRouter, error) {
	ctx, cancel := context.WithCancel(context.Background())
	router := &RlncRouter{
		ctx:    ctx,
		cancel: cancel,

		messageIDFunc: hashSha256, // SHA-256 gives good distribution for message IDs

		peers:  make(map[peer.ID]pubsub.TopicSendFunc),
		chunks: make(map[string][]Chunk),
	}

	// Use a large prime field by default: 2^256 + 297 (prime)
	// This gives us good security properties for linear combinations
	p := new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil)
	p.Add(p, big.NewInt(297))
	router.field = field.NewPrimeField(p)

	router.cond = sync.NewCond(&router.mutex)

	// Set reasonable defaults for most use cases
	router.params = RlncParams{
		MessageChunkSize:   1024, // 1KB chunks are a good balance
		NetworkChunkSize:   1030, // Slightly larger to accommodate field encoding
		MaxCoefficientBits: 16,   // 16-bit coefficients give good randomness
		PublishMultiplier:  2,    // Send 2x redundancy when publishing
		ForwardMultiplier:  2,    // Forward 2 chunks per useful chunk received
		ElementsPerChunk:   8 * router.params.MessageChunkSize / router.field.BitsPerDataElement(),
	}

	for _, opt := range opts {
		err := opt(router)
		if err != nil {
			return nil, err
		}
	}

	// Calculate and validate bit allocations for field element encoding
	// This is critical - we need to ensure data fits properly in field elements
	router.messageBitsPerElement = (8 * router.params.MessageChunkSize) / router.params.ElementsPerChunk
	if router.messageBitsPerElement > router.field.BitsPerDataElement() {
		return nil, fmt.Errorf("(8*MessageChunkSize)/ElementsPerChunk (%d) is too high for the field", router.messageBitsPerElement)
	}
	router.networkBitsPerElement = (8 * router.params.NetworkChunkSize) / router.params.ElementsPerChunk
	if router.networkBitsPerElement < router.field.BitsPerElement() {
		return nil, fmt.Errorf("(8*NetworkChunkSize)/ElementsPerChunk (%d) is too low for the field", router.networkBitsPerElement)
	}

	return router, nil
}

// WithRlncParams sets custom RLNC parameters - validates ElementsPerChunk alignment
func WithRlncParams(params RlncParams) RlncOption {
	return func(router *RlncRouter) error {
		if (8*params.MessageChunkSize)%params.ElementsPerChunk != 0 {
			return fmt.Errorf("ElementsPerChunk (%d) must divide 8*MessageChunkSize (%d)",
				params.ElementsPerChunk, 8*params.MessageChunkSize)
		}
		router.params = params
		return nil
	}
}

// WithField sets a custom finite field for linear algebra operations
func WithField(finiteField field.Field) RlncOption {
	return func(router *RlncRouter) error {
		router.field = finiteField
		return nil
	}
}

// WithChunkVerifyAlgorithm plugs in custom chunk verification (e.g., commitments, signatures)
func WithChunkVerifyAlgorithm(verifier ChunkVerifyAlgorithm) RlncOption {
	return func(router *RlncRouter) error {
		router.verifier = verifier
		return nil
	}
}

// WithMessageIdFn sets a custom message ID function (default is SHA-256)
func WithMessageIdFn(messageIDFunc MsgIdFunc) RlncOption {
	return func(router *RlncRouter) error {
		router.messageIDFunc = messageIDFunc
		return nil
	}
}

// Chunk represents a network coding chunk with its linear combination coefficients
type Chunk struct {
	Data   []byte          // The actual chunk data (encoded for network transmission)
	Coeffs []field.Element // Coefficients for the linear combination this chunk represents
	Extra  []byte          // Optional extra data (verification info, signatures, etc.)
}

// RlncRouter implements Random Linear Network Coding for efficient broadcast
// The core idea: encode messages as linear combinations over finite fields,
// allowing any k linearly independent chunks to reconstruct k original chunks
type RlncRouter struct {
	ctx    context.Context
	cancel context.CancelFunc

	verifier ChunkVerifyAlgorithm // Optional verification (can be nil)
	field    field.Field          // Finite field for all linear algebra

	mutex sync.Mutex // Protects all mutable state below
	cond  *sync.Cond // For blocking on message receipt

	params        RlncParams // Configuration parameters
	messageIDFunc MsgIdFunc  // How we identify messages

	messageBitsPerElement int // Bits per field element for message data
	networkBitsPerElement int // Bits per field element for network data

	peers  map[peer.ID]pubsub.TopicSendFunc // Active peer connections
	chunks map[string][]Chunk               // Received chunks by message ID

	received [][]byte // Queue of fully reconstructed messages
}

type RlncParams struct {
	// Message chunk size in bytes. When the message is larger than this size, it will be chunked, so
	// every chunk will be of this size.
	MessageChunkSize int
	// Network chunk size in bytes. This is the actual size of chunks sent into the network.
	NetworkChunkSize int
	// The number of field elements per chunk.
	ElementsPerChunk int
	// Max coeficient bits used in linear combinations.
	MaxCoefficientBits int
	// Multiplier of the number of chunks sent out in total, when publish. For example, when
	// the multiplier is 2 and the number of chunks of the published message is 8, there will
	// be 2*8 = 16 chunks sent in total.
	PublishMultiplier int
	// Every time a new useful chunk is receive, we send out as many new chunks as this number.
	ForwardMultiplier int
}

// Publish takes a message, splits it into chunks, and broadcasts linear combinations
// The message size must be a multiple of MessageChunkSize for simplicity
func (router *RlncRouter) Publish(message []byte) error {
	if len(message)%router.params.MessageChunkSize != 0 {
		return fmt.Errorf("the size of the message (%d) must be a multiple of the chunk size (%d)",
			len(message), router.params.MessageChunkSize)
	}

	messageBuffer := message
	messageID := router.messageIDFunc(messageBuffer)

	// Split message into fixed-size chunks and prepare for network encoding
	var chunks []Chunk
	var chunkBuffers [][]byte
	for chunkIndex := 0; len(messageBuffer) > 0; chunkIndex++ {
		chunkData := messageBuffer[:router.params.MessageChunkSize]

		// Convert to field elements and re-encode for network transmission
		fieldElements := field.SplitBitsToFieldElements(chunkData, router.messageBitsPerElement, router.field)
		chunkData = field.FieldElementsToBytes(fieldElements, router.networkBitsPerElement)

		chunks = append(chunks, Chunk{
			Data: chunkData,
		})
		chunkBuffers = append(chunkBuffers, chunkData)
		messageBuffer = messageBuffer[router.params.MessageChunkSize:]
	}

	// Generate extra fields
	if router.verifier != nil {
		extras, err := router.verifier.GenerateExtras(chunkBuffers)
		if err != nil {
			return err
		}
		for i := range chunks {
			chunks[i].Extra = extras[i]
		}
	}
	// Set up identity matrix for coefficients - each chunk initially represents itself
	// This is the foundation that allows reconstruction via linear algebra
	coefficients := make([]field.Element, len(chunks))
	for i := range chunks {
		coefficients[i] = router.field.Zero()
	}
	for i := range chunks {
		coefficients[i] = router.field.One()          // chunk i has coefficient 1 for itself
		chunks[i].Coeffs = slices.Clone(coefficients) // copy the coefficient vector
		coefficients[i] = router.field.Zero()         // reset for next chunk
	}
	router.chunks[messageID] = chunks

	router.mutex.Lock()
	router.received = append(router.received, message)
	router.cond.Signal()

	sendFuncs := slices.Collect(maps.Values(router.peers))
	// Shuffle the peers
	mrand.Shuffle(len(sendFuncs), func(i, j int) {
		sendFuncs[i], sendFuncs[j] = sendFuncs[j], sendFuncs[i]
	})
	router.mutex.Unlock()

	sendFuncIndex := 0

	// Broadcast random linear combinations to peers (round-robin for load balancing)
	for i := 0; i < len(chunks)*router.params.PublishMultiplier; i++ {
		// Create a random linear combination for this transmission
		combinedChunk, err := router.combineChunks(chunks)
		if err != nil {
			log.Warnf("error publishing; %s", err)
			// Skipping to the next peer
			continue
		}
		// send the combined chunk to that peer
		rpcChunk := &pb.RlncRpc_Chunk{
			MessageID: &messageID,
			Data:      combinedChunk.Data,
			Extra:     combinedChunk.Extra,
		}
		for _, coefficient := range combinedChunk.Coeffs {
			rpcChunk.Coefficients = append(rpcChunk.Coefficients, coefficient.Bytes())
		}
		sendFuncs[sendFuncIndex](&pb.TopicRpc{
			Rlnc: &pb.RlncRpc{
				Chunks: []*pb.RlncRpc_Chunk{rpcChunk},
			},
		})

		sendFuncIndex++
		sendFuncIndex %= len(sendFuncs)
	}
	return nil
}

// sendChunk forwards a random linear combination to help message propagation
func (router *RlncRouter) sendChunk(messageID string) {
	router.mutex.Lock()
	defer router.mutex.Unlock()

	combinedChunk, err := router.combineChunks(router.chunks[messageID])
	if err != nil {
		log.Warnf("error forwarding; %s", err)
		return
	}
	// Send the combined chunk to that peer
	rpcChunk := &pb.RlncRpc_Chunk{
		MessageID: &messageID,
		Data:      combinedChunk.Data,
		Extra:     combinedChunk.Extra,
	}
	for _, coefficient := range combinedChunk.Coeffs {
		rpcChunk.Coefficients = append(rpcChunk.Coefficients, coefficient.Bytes())
	}
	// Pick a random peer
	peerIndex := slices.Collect(maps.Keys(router.peers))[mrand.Intn(len(router.peers))]
	// Send the rpc
	router.peers[peerIndex](&pb.TopicRpc{
		Rlnc: &pb.RlncRpc{
			Chunks: []*pb.RlncRpc_Chunk{rpcChunk},
		},
	})
}

// notifyMessage tries to reconstruct a complete message and queue it for delivery
func (router *RlncRouter) notifyMessage(messageID string) {
	router.mutex.Lock()
	defer router.mutex.Unlock()

	reconstructedMessage, err := router.reconstructMessage(router.chunks[messageID])
	if err != nil {
		// Quietly return
		return
	}

	router.received = append(router.received, reconstructedMessage)
	router.cond.Signal()
}

// combineChunks creates a random linear combination of chunks - the heart of RLNC
// Each chunk gets a random coefficient, and we compute the weighted sum
func (router *RlncRouter) combineChunks(chunks []Chunk) (Chunk, error) {
	var randomFactors []field.Element
	accumulator := make([]field.Element, router.params.ElementsPerChunk)
	for i := range accumulator {
		accumulator[i] = router.field.Zero()
	}

	for _, chunk := range chunks {
		fieldElements := field.SplitBitsToFieldElements(chunk.Data, router.networkBitsPerElement, router.field)
		// Pick a random coefficient for this chunk
		randomFactor, err := router.field.RandomMax(router.params.MaxCoefficientBits)
		if err != nil {
			return Chunk{}, err
		}
		randomFactors = append(randomFactors, randomFactor)
		for i, element := range fieldElements {
			// accumulator[i] = accumulator[i] + randomFactor * element
			accumulator[i] = accumulator[i].Add(randomFactor.Mul(element))
		}
	}

	// Update the coefficient vectors: new_coeffs[i] = sum(factor[j] * old_coeffs[j][i])
	// This maintains the linear algebra invariant for reconstruction
	combinedCoefficients := make([]field.Element, len(chunks[0].Coeffs))
	for i := range combinedCoefficients {
		combinedCoefficients[i] = router.field.Zero()
		for j, randomFactor := range randomFactors {
			// combinedCoefficients[i] = combinedCoefficients[i] + randomFactor * chunks[j].Coeffs[i]
			combinedCoefficients[i] = combinedCoefficients[i].Add(randomFactor.Mul(chunks[j].Coeffs[i]))
		}
	}

	// Let the verifier handle combination of extra data (if any)
	var extraData []byte
	if router.verifier != nil {
		var err error
		extraData, err = router.verifier.CombineExtras(chunks, randomFactors)
		if err != nil {
			return Chunk{}, err
		}
	}

	combinedData := field.FieldElementsToBytes(accumulator, router.networkBitsPerElement)
	combinedChunk := Chunk{
		Data:   combinedData,
		Coeffs: combinedCoefficients,
		Extra:  extraData,
	}
	return combinedChunk, nil
}

// reconstructMessage recovers the original message by solving the linear system
// This is where the mathematical magic happens - linear algebra FTW!
func (router *RlncRouter) reconstructMessage(chunks []Chunk) ([]byte, error) {
	/*
	   To recover the original vectors v₁, ..., vₙ from n random linear combinations r₁, ..., rₙ over a field, we assume
	   the random linear combinations are linearly independent, i.e., the matrix of coefficients used to produce the combinations
	   is invertible. So:

	   Let V be an n × m matrix where each row is a vector vᵢ.

	   Let A be an n × n matrix of random coefficients.

	   Let R = A · V, where each row of R is rᵢ.

	   To recover V, compute A⁻¹ · R.
	*/
	var A [][]field.Element
	var R [][]field.Element

	for _, chunk := range chunks {
		r := field.SplitBitsToFieldElements(chunk.Data, router.networkBitsPerElement, router.field)
		A = append(A, chunk.Coeffs)
		R = append(R, r)
	}
	V, err := field.RecoverVectors(A, R, router.field)
	if err != nil {
		return nil, err
	}

	var messageBuffer []byte
	for _, v := range V {
		chunkData := field.FieldElementsToBytes(v, router.messageBitsPerElement)
		messageBuffer = slices.Concat(messageBuffer, chunkData)
	}
	return messageBuffer, nil
}

// Next blocks until a complete message is reconstructed and returns it
func (router *RlncRouter) Next(ctx context.Context) ([]byte, error) {
	router.mutex.Lock()
	defer router.mutex.Unlock()

	if len(router.received) > 0 {
		message := router.received[0]
		router.received = router.received[1:]
		return message, nil
	}

	unregisterAfterFunc := context.AfterFunc(ctx, func() {
		// Wake up all the waiting routines. The only routine that corresponds
		// to this Next call will return from the function. Note that this can
		// be expensive, if there are too many waiting Wait calls.
		router.cond.Broadcast()
	})
	defer unregisterAfterFunc()

	for len(router.received) == 0 {
		router.cond.Wait()
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("the call has been cancelled")
		case <-router.ctx.Done():
			return nil, fmt.Errorf("the router has been closed")
		default:
		}
	}
	message := router.received[0]
	router.received = router.received[1:]
	return message, nil
}

// AddPeer registers a new peer for message distribution
func (router *RlncRouter) AddPeer(peerID peer.ID, sendFunc pubsub.TopicSendFunc) {
	router.mutex.Lock()
	defer router.mutex.Unlock()

	router.peers[peerID] = sendFunc
}

// RemovePeer unregisters a peer (cleanup when they disconnect)
func (router *RlncRouter) RemovePeer(peerID peer.ID) {
	router.mutex.Lock()
	defer router.mutex.Unlock()

	delete(router.peers, peerID)
}

// HandleIncomingRPC processes chunks from peers and manages forwarding
// This is where we check for linear independence and trigger reconstruction
func (router *RlncRouter) HandleIncomingRPC(peerID peer.ID, topicRPC *pb.TopicRpc) {
	router.mutex.Lock()

	rlncRPC := topicRPC.GetRlnc()
	messagesToNotify := make(map[string]struct{})
	messagesToSend := make(map[string]int) // The number of chunks to send out indexed by message IDs
	for _, chunkRPC := range rlncRPC.GetChunks() {
		messageID := chunkRPC.GetMessageID()
		chunkData := chunkRPC.GetData()
		var coefficients []field.Element
		for _, coefficientBytes := range chunkRPC.GetCoefficients() {
			coefficients = append(coefficients, router.field.FromBytes(coefficientBytes))
		}
		if len(chunkData) != router.params.NetworkChunkSize {
			continue
		}

		// Key optimization: reject chunks that don't add new information
		// Only linearly independent chunks help us make progress toward reconstruction
		var coefficientVectors [][]field.Element
		for _, existingChunk := range router.chunks[messageID] {
			// TODO: Handle the case that the numbers of coefficients are different among
			// chunks. I don't know yet what to do.
			coefficientVectors = append(coefficientVectors, existingChunk.Coeffs)
		}
		coefficientVectors = append(coefficientVectors, coefficients)
		if !field.IsLinearlyIndependent(coefficientVectors, router.field) {
			continue
		}

		newChunk := Chunk{
			Data:   chunkData,
			Coeffs: coefficients,
			Extra:  chunkRPC.GetExtra(),
		}
		// Give the application a chance to verify this chunk (signatures, commitments, etc.)
		if router.verifier != nil && !router.verifier.Verify(&newChunk) {
			continue
		}
		// Remember the chunk
		router.chunks[messageID] = append(router.chunks[messageID], newChunk)

		// Useful chunk! Forward it to help network propagation
		messagesToSend[messageID] += router.params.ForwardMultiplier
		// Do we have enough linearly independent chunks to reconstruct?
		// If so, try to rebuild the original message
		// TODO: This assumes all chunks have the same coefficient vector length
		if len(router.chunks[messageID]) >= len(coefficients) {
			messagesToNotify[messageID] = struct{}{}
		}
	}
	router.mutex.Unlock()

	for messageID, chunkCount := range messagesToSend {
		for i := 0; i < chunkCount; i++ {
			router.sendChunk(messageID)
		}
	}
	for messageID := range messagesToNotify {
		router.notifyMessage(messageID)
	}
}

// Close shuts down the router and wakes up any waiting Next() calls
func (router *RlncRouter) Close() error {
	router.cancel()
	router.cond.Broadcast()
	return nil
}

// hashSha256 is our default message ID function - SHA-256 hex string
func hashSha256(data []byte) string {
	hasher := sha256.New()
	hasher.Write(data)
	return fmt.Sprintf("%x", hasher.Sum(nil))
}
