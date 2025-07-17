package floodsub

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ethp2p/eth-ec-broadcast/pb"
	"github.com/ethp2p/eth-ec-broadcast/pubsub"

	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	TimeCacheDuration = 120 * time.Second
)

// MsgIdFunc generates unique message IDs from message data
type MsgIdFunc func([]byte) string

// FloodSubRouter implements message flooding for topic broadcast
type FloodSubRouter struct {
	ctx    context.Context
	cancel context.CancelFunc

	mutex sync.Mutex // Protects all fields below
	cond  *sync.Cond // Notifies waiting consumers

	peerSendFuncs    map[peer.ID]pubsub.TopicSendFunc // Send functions for each peer
	messageIdFunc    MsgIdFunc                        // Function to generate message IDs
	seenMessages     *TimeCache                       // Cache of seen message IDs
	receivedMessages [][]byte                         // Buffer of received messages
}

// NewFloodsub creates a new FloodSub router with the given message ID function
func NewFloodsub(messageIdFunc MsgIdFunc) *FloodSubRouter {
	ctx, cancel := context.WithCancel(context.Background())
	router := &FloodSubRouter{
		ctx:    ctx,
		cancel: cancel,

		peerSendFuncs: make(map[peer.ID]pubsub.TopicSendFunc),
		messageIdFunc: messageIdFunc,
		seenMessages:  NewTimeCache(TimeCacheDuration),
	}
	router.cond = sync.NewCond(&router.mutex)
	return router
}

// Publish broadcasts a message to all connected peers
func (router *FloodSubRouter) Publish(messageData []byte) error {
	router.handleMessages([][]byte{messageData})
	return nil
}

// Next returns the next received message, blocking until one is available
func (router *FloodSubRouter) Next(ctx context.Context) ([]byte, error) {
	router.mutex.Lock()
	defer router.mutex.Unlock()

	// Return immediately if we have buffered messages
	if len(router.receivedMessages) > 0 {
		messageData := router.receivedMessages[0]
		router.receivedMessages = router.receivedMessages[1:]
		return messageData, nil
	}

	// Set up context cancellation to wake up waiters
	unregisterAfterFunc := context.AfterFunc(ctx, func() {
		// Wake up all waiting routines when context is cancelled
		router.cond.Broadcast()
	})
	defer unregisterAfterFunc()

	// Wait for messages to arrive
	for len(router.receivedMessages) == 0 {
		router.cond.Wait()
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("the call has been cancelled")
		case <-router.ctx.Done():
			return nil, fmt.Errorf("the router has been closed")
		default:
		}
	}
	messageData := router.receivedMessages[0]
	router.receivedMessages = router.receivedMessages[1:]
	return messageData, nil
}

// AddPeer registers a new peer with its send function
func (router *FloodSubRouter) AddPeer(peerID peer.ID, sendFunc pubsub.TopicSendFunc) {
	router.mutex.Lock()
	defer router.mutex.Unlock()

	router.peerSendFuncs[peerID] = sendFunc
}

// RemovePeer removes a peer from the router
func (router *FloodSubRouter) RemovePeer(peerID peer.ID) {
	router.mutex.Lock()
	defer router.mutex.Unlock()

	delete(router.peerSendFuncs, peerID)
}

// HandleIncomingRPC processes incoming RPC messages from peers
func (router *FloodSubRouter) HandleIncomingRPC(fromPeer peer.ID, topicRPC *pb.TopicRpc) {
	floodsubRPC := topicRPC.GetFloodsub()
	router.handleMessages(floodsubRPC.GetMessages())
}

// Close shuts down the router and releases resources
func (router *FloodSubRouter) Close() error {
	router.cancel()
	router.cond.Broadcast()
	router.seenMessages.Close()
	return nil
}

// handleMessages processes and forwards new messages to peers
func (router *FloodSubRouter) handleMessages(messages [][]byte) {
	router.mutex.Lock()
	// Copy send functions to avoid holding mutex during network calls
	var sendFuncs []pubsub.TopicSendFunc
	for _, sendFunc := range router.peerSendFuncs {
		sendFuncs = append(sendFuncs, sendFunc)
	}

	rpc := &pb.TopicRpc{
		Floodsub: &pb.FloodsubRpc{},
	}
	// Process each message and deduplicate using seen cache
	for _, messageData := range messages {
		messageID := router.messageIdFunc(messageData)
		if !router.seenMessages.Has(messageID) {
			router.seenMessages.Add(messageID)
			router.receivedMessages = append(router.receivedMessages, messageData)
			router.cond.Signal()
			rpc.Floodsub.Messages = append(rpc.Floodsub.Messages, messageData)
		}
	}

	router.mutex.Unlock()
	// Forward new messages to all connected peers
	if len(rpc.Floodsub.Messages) > 0 {
		for _, sendFunc := range sendFuncs {
			sendFunc(rpc)
		}
	}
}
