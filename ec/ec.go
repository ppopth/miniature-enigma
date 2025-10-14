package ec

import (
	"context"
	"crypto/sha256"
	"fmt"
	"maps"
	mrand "math/rand"
	"slices"
	"sync"

	"github.com/ethp2p/eth-ec-broadcast/ec/encode"
	"github.com/ethp2p/eth-ec-broadcast/pb"
	"github.com/ethp2p/eth-ec-broadcast/pubsub"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/peer"
)

var log = logging.Logger("ec")

// MsgIdFunc generates unique identifiers for messages
type MsgIdFunc func([]byte) string

// EcOption configures an EC router during construction
type EcOption func(*EcRouter) error

// NewEcRouter creates a new EC router with the given encoder and applies options
func NewEcRouter(encoder encode.Encoder, opts ...EcOption) (*EcRouter, error) {
	if encoder == nil {
		return nil, fmt.Errorf("encoder is required")
	}

	ctx, cancel := context.WithCancel(context.Background())
	router := &EcRouter{
		ctx:    ctx,
		cancel: cancel,

		encoder:       encoder,
		messageIDFunc: hashSha256, // SHA-256 gives good distribution for message IDs

		peers:                 make(map[peer.ID]pubsub.TopicSendFunc),
		peerCompletedMessages: make(map[peer.ID]map[string]struct{}),
		notified:              make(map[string]struct{}),
		completionSignalsSent: make(map[string]struct{}),
	}

	router.cond = sync.NewCond(&router.mutex)

	// Set reasonable defaults for router-specific parameters
	router.params = EcParams{
		PublishMultiplier: 2, // Send 2x redundancy when publishing
		ForwardMultiplier: 2, // Forward 2 chunks per innovative chunk received
		// DisableCompletionSignal defaults to false (signals are sent by default)
	}

	for _, opt := range opts {
		err := opt(router)
		if err != nil {
			return nil, err
		}
	}

	return router, nil
}

// WithEcParams sets custom EC parameters
func WithEcParams(params EcParams) EcOption {
	return func(router *EcRouter) error {
		router.params = params
		return nil
	}
}

// WithMessageIdFn sets a custom message ID function (default is SHA-256)
func WithMessageIdFn(messageIDFunc MsgIdFunc) EcOption {
	return func(router *EcRouter) error {
		router.messageIDFunc = messageIDFunc
		return nil
	}
}

// EcRouter implements Erasure Coding for efficient broadcast
// The core idea: encode messages as linear combinations over finite fields,
// allowing any k linearly independent chunks to reconstruct k original chunks
type EcRouter struct {
	ctx    context.Context
	cancel context.CancelFunc

	encoder encode.Encoder // Encoder for erasure coding operations

	mutex sync.Mutex // Protects all mutable state below
	cond  *sync.Cond // For blocking on message receipt

	params        EcParams  // Configuration parameters
	messageIDFunc MsgIdFunc // How we identify messages

	peers                 map[peer.ID]pubsub.TopicSendFunc // Active peer connections
	peerCompletedMessages map[peer.ID]map[string]struct{}  // Track which peers have completed which messages

	received              [][]byte            // Queue of fully reconstructed messages
	notified              map[string]struct{} // Track which messages have been notified to prevent duplicates
	completionSignalsSent map[string]struct{} // Track which messages have had completion signals sent

	preventedChunks int // Count of chunks prevented from being sent due to completion signals
}

type EcParams struct {
	// Multiplier of the number of chunks sent out in total, when publish. For example, when
	// the multiplier is 2 and the number of chunks of the published message is 8, there will
	// be 2*8 = 16 chunks sent in total.
	PublishMultiplier int
	// Every time a new innovative chunk is receive, we send out as many new chunks as this number.
	ForwardMultiplier int
	// Whether to disable sending completion signals to peers when a message is fully reconstructed.
	// By default (false), completion signals are sent to allow peers to stop sending chunks for
	// completed messages, reducing unused chunk overhead. Set to true to disable this feature.
	DisableCompletionSignal bool
}

// Publish takes a message, splits it into chunks, and broadcasts linear combinations
func (router *EcRouter) Publish(message []byte) error {
	messageID := router.messageIDFunc(message)

	// Use encoder to generate chunks
	numChunks, err := router.encoder.GenerateThenAddChunks(messageID, message)
	if err != nil {
		return fmt.Errorf("failed to generate chunks: %w", err)
	}

	router.mutex.Lock()
	router.received = append(router.received, message)
	router.cond.Signal()

	sendFuncs := slices.Collect(maps.Values(router.peers))

	// Handle case when there are no peers connected
	if len(sendFuncs) == 0 {
		router.mutex.Unlock()
		log.Debugf("Published message %s but no peers connected - message stored locally", messageID[:8])
		return nil
	}

	// Shuffle the peers
	mrand.Shuffle(len(sendFuncs), func(i, j int) {
		sendFuncs[i], sendFuncs[j] = sendFuncs[j], sendFuncs[i]
	})
	router.mutex.Unlock()

	sendFuncIndex := 0

	// Broadcast random linear combinations to peers (round-robin for load balancing)
	for i := 0; i < numChunks*router.params.PublishMultiplier; i++ {
		// Create a random linear combination for this transmission
		combinedChunk, err := router.encoder.EmitChunk(messageID)
		if err != nil {
			log.Warnf("error publishing; %s", err)
			// Skipping to the next peer
			continue
		}
		// send the combined chunk to that peer
		rpcChunk := &pb.EcRpc_Chunk{
			MessageID: &messageID,
			Data:      combinedChunk.Data(),
			Extra:     combinedChunk.EncodeExtra(),
		}
		sendFuncs[sendFuncIndex](&pb.TopicRpc{
			Ec: &pb.EcRpc{
				Chunks: []*pb.EcRpc_Chunk{rpcChunk},
			},
		})

		sendFuncIndex++
		sendFuncIndex %= len(sendFuncs)
	}
	return nil
}

// broadcastCompletionSignal notifies all peers that this node has completed reconstruction for a message.
// This allows peers to stop sending chunks for this message, reducing unused chunk overhead.
func (router *EcRouter) broadcastCompletionSignal(messageID string) {
	router.mutex.Lock()
	// Get a copy of peer send functions to broadcast completion signal
	peerSendFuncs := make(map[peer.ID]pubsub.TopicSendFunc)
	for peerID, sendFunc := range router.peers {
		peerSendFuncs[peerID] = sendFunc
	}
	router.mutex.Unlock()

	// Broadcast completion signal to all peers (outside the lock to avoid deadlocks)
	log.Debugf("Broadcasting completion signal for message %s to %d peers", messageID[:8], len(peerSendFuncs))
	for peerID, sendFunc := range peerSendFuncs {
		sendFunc(&pb.TopicRpc{
			Ec: &pb.EcRpc{
				CompletedMessages: []string{messageID},
			},
		})
		log.Debugf("Sent completion signal for message %s to peer %s", messageID[:8], peerID.String()[:8])
	}
}

// sendChunk forwards a random linear combination to help message propagation
func (router *EcRouter) sendChunk(messageID string, excludePeer peer.ID) {
	router.mutex.Lock()
	defer router.mutex.Unlock()

	// Check if we have any peers to send to
	if len(router.peers) == 0 {
		log.Debugf("Cannot forward chunk for message %s - no peers connected", messageID[:8])
		return
	}

	// Get all peers except the one we should exclude
	var availablePeers []peer.ID
	for peerID := range router.peers {
		if peerID != excludePeer {
			availablePeers = append(availablePeers, peerID)
		}
	}

	// Check if we have any available peers after exclusion
	if len(availablePeers) == 0 {
		log.Debugf("Cannot forward chunk for message %s - only sender peer connected", messageID[:8])
		return
	}

	combinedChunk, err := router.encoder.EmitChunk(messageID)
	if err != nil {
		log.Warnf("error forwarding; %s", err)
		return
	}
	// Send the combined chunk to that peer
	rpcChunk := &pb.EcRpc_Chunk{
		MessageID: &messageID,
		Data:      combinedChunk.Data(),
		Extra:     combinedChunk.EncodeExtra(),
	}
	// Pick a random peer (excluding the sender)
	selectedPeer := availablePeers[mrand.Intn(len(availablePeers))]

	// Check if this peer has already completed this message right before sending
	if _, completed := router.peerCompletedMessages[selectedPeer][messageID]; completed {
		// This chunk would have been sent but was prevented by completion signal
		router.preventedChunks++
		log.Infof("Prevented chunk send for message %s to peer %s (completion signal received)", messageID[:8], selectedPeer.String()[:8])
		return
	}

	// Send the rpc
	router.peers[selectedPeer](&pb.TopicRpc{
		Ec: &pb.EcRpc{
			Chunks: []*pb.EcRpc_Chunk{rpcChunk},
		},
	})
}

// notifyMessage tries to reconstruct a complete message and queue it for delivery
func (router *EcRouter) notifyMessage(messageID string) {
	router.mutex.Lock()
	defer router.mutex.Unlock()

	// Check if we've already notified this message
	if _, exists := router.notified[messageID]; exists {
		return
	}

	reconstructedMessage, err := router.encoder.ReconstructMessage(messageID)
	if err != nil {
		// Quietly return
		return
	}

	// Mark message as notified and add to queue
	router.notified[messageID] = struct{}{}
	router.received = append(router.received, reconstructedMessage)
	router.cond.Signal()
}

// Next blocks until a complete message is reconstructed and returns it
func (router *EcRouter) Next(ctx context.Context) ([]byte, error) {
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
func (router *EcRouter) AddPeer(peerID peer.ID, sendFunc pubsub.TopicSendFunc) {
	router.mutex.Lock()
	defer router.mutex.Unlock()

	router.peers[peerID] = sendFunc
	router.peerCompletedMessages[peerID] = make(map[string]struct{})
}

// RemovePeer unregisters a peer (cleanup when they disconnect)
func (router *EcRouter) RemovePeer(peerID peer.ID) {
	router.mutex.Lock()
	defer router.mutex.Unlock()

	delete(router.peers, peerID)
	delete(router.peerCompletedMessages, peerID)
}

// HandleIncomingRPC processes chunks from peers and manages forwarding
// This is where we check for linear independence and trigger reconstruction
func (router *EcRouter) HandleIncomingRPC(peerID peer.ID, topicRPC *pb.TopicRpc) {
	router.mutex.Lock()

	ecRPC := topicRPC.GetEc()

	// Process completion signals from peer
	for _, completedMessageID := range ecRPC.GetCompletedMessages() {
		log.Debugf("Received completion signal for message %s from peer %s", completedMessageID[:8], peerID.String()[:8])
		router.peerCompletedMessages[peerID][completedMessageID] = struct{}{}
	}

	messagesToNotify := make(map[string]struct{})
	messagesToSend := make(map[string]int) // The number of chunks to send out indexed by message IDs
	messagesToSendCompletionSignal := make(map[string]struct{})
	disableCompletionSignal := router.params.DisableCompletionSignal

	for _, chunkRPC := range ecRPC.GetChunks() {
		messageID := chunkRPC.GetMessageID()
		chunkData := chunkRPC.GetData()
		extraData := chunkRPC.GetExtra()

		log.Debugf("Received chunk for message %s from peer %s", messageID[:8], peerID.String()[:8])

		// Decode the chunk using the encoder
		decodedChunk, err := router.encoder.DecodeChunk(messageID, chunkData, extraData)
		if err != nil {
			log.Debugf("Failed to decode chunk for message %s from peer %s: %v", messageID[:8], peerID.String()[:8], err)
			continue
		}

		// Use encoder to verify and add the chunk
		if !router.encoder.VerifyThenAddChunk(decodedChunk) {
			log.Debugf("Chunk verification failed for message %s from peer %s (duplicate or invalid)", messageID[:8], peerID.String()[:8])
			continue
		}

		log.Debugf("Valid chunk accepted for message %s from peer %s", messageID[:8], peerID.String()[:8])

		// Innovative chunk! Forward it to help network propagation
		messagesToSend[messageID] += router.params.ForwardMultiplier

		chunkCount := router.encoder.GetChunkCount(messageID)
		minChunks := router.encoder.GetMinChunksForReconstruction(messageID)
		chunksBeforeCompletion := router.encoder.GetChunksBeforeCompletion(messageID)

		// Check if we have enough chunks to reconstruct the message
		if chunkCount >= minChunks {
			if _, err := router.encoder.ReconstructMessage(messageID); err == nil {
				log.Debugf("Successfully reconstructed message %s", messageID[:8])
				messagesToNotify[messageID] = struct{}{}
			} else {
				log.Debugf("Failed to reconstruct message %s: %v", messageID[:8], err)
			}
		}

		// Check if we should send completion signal (after receiving all expected chunks)
		if !disableCompletionSignal && chunkCount >= chunksBeforeCompletion {
			// Check if we've already sent completion signal for this message
			if _, alreadySent := router.completionSignalsSent[messageID]; !alreadySent {
				messagesToSendCompletionSignal[messageID] = struct{}{}
				router.completionSignalsSent[messageID] = struct{}{}
			}
		}
	}
	router.mutex.Unlock()

	for messageID, chunkCount := range messagesToSend {
		for i := 0; i < chunkCount; i++ {
			router.sendChunk(messageID, peerID)
		}
	}
	for messageID := range messagesToNotify {
		router.notifyMessage(messageID)
	}
	for messageID := range messagesToSendCompletionSignal {
		router.broadcastCompletionSignal(messageID)
	}
}

// Close shuts down the router and wakes up any waiting Next() calls
func (router *EcRouter) Close() error {
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
