package pubsub

import (
	"context"
	"fmt"
	"sync"

	"github.com/ethp2p/eth-ec-broadcast/host"
	"github.com/ethp2p/eth-ec-broadcast/pb"
	"github.com/gogo/protobuf/proto"

	"github.com/libp2p/go-libp2p/core/peer"
)

// TopicSendFunc is a function that sends topic RPC messages
type TopicSendFunc func(*pb.TopicRpc)

// PeerSend holds a peer ID and its send function
type PeerSend struct {
	ID       peer.ID
	SendFunc TopicSendFunc
}

// Topic manages a single pubsub topic and its subscriptions
type Topic struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mutex sync.Mutex // Protects all fields below

	router    Router // Message routing implementation
	topicName string // Name of this topic

	subscriptions map[*Subscription]struct{} // Active subscriptions
	peers         map[peer.ID]host.Sender    // Connected peers
	listeners     []TopicEventListener       // Event listeners
}

// newTopic creates a new topic with the given router
func newTopic(topicName string, router Router) *Topic {
	ctx, cancel := context.WithCancel(context.Background())
	topic := &Topic{
		ctx:    ctx,
		cancel: cancel,

		router:        router,
		topicName:     topicName,
		subscriptions: make(map[*Subscription]struct{}),
		peers:         make(map[peer.ID]host.Sender),
	}

	topic.wg.Add(1)
	go topic.background()

	return topic
}

// String returns the topic name
func (t *Topic) String() string {
	return t.topicName
}

// Close shuts down the topic and all its subscriptions
func (t *Topic) Close() error {
	t.mutex.Lock()
	// Cancel context to prevent new operations
	t.cancel()
	// Copy subscriptions to avoid deadlock during Close()
	var subscriptionsToClose []*Subscription
	for subscription := range t.subscriptions {
		subscriptionsToClose = append(subscriptionsToClose, subscription)
	}
	t.mutex.Unlock()

	// Close all subscriptions without holding the mutex
	for _, subscription := range subscriptionsToClose {
		subscription.Close()
	}

	if t.router != nil {
		t.router.Close()
	}
	t.wg.Wait()
	return nil
}

// Publish publishes a message to the topic
func (t *Topic) Publish(data []byte) error {
	return t.router.Publish(data)
}

// Subscribe creates a new subscription to this topic
func (t *Topic) Subscribe() (*Subscription, error) {
	select {
	case <-t.ctx.Done():
		return nil, fmt.Errorf("topic has been closed")
	default:
	}

	subscription := newSubscription(t)

	t.mutex.Lock()
	defer t.mutex.Unlock()
	// If this is the first subscription, notify about subscribe event
	if len(t.subscriptions) == 0 {
		t.notifyEvent(TopicEventSubscribe)
	}
	t.subscriptions[subscription] = struct{}{}

	return subscription, nil
}

// removeSubscription removes a subscription from the topic
func (t *Topic) removeSubscription(subscription *Subscription) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	delete(t.subscriptions, subscription)
	// If no more subscriptions, notify about unsubscribe event
	if len(t.subscriptions) == 0 {
		t.notifyEvent(TopicEventUnsubscribe)
	}
}

// TopicEventListener receives topic events
type TopicEventListener func(TopicEvent)

// TopicEvent represents topic subscription events
type TopicEvent int

const (
	TopicEventSubscribe = iota
	TopicEventUnsubscribe
)

// AddEventListener registers a listener for topic events
func (t *Topic) AddEventListener(listener TopicEventListener) {
	select {
	case <-t.ctx.Done():
		return
	default:
	}

	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.listeners = append(t.listeners, listener)
}

// notifyEvent sends an event to all listeners (must be called with mutex held)
func (t *Topic) notifyEvent(event TopicEvent) {
	for _, listener := range t.listeners {
		listener(event)
	}
}

// addPeer adds a peer to this topic
func (t *Topic) addPeer(peerID peer.ID, sender host.Sender) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.peers[peerID] = sender
	t.router.AddPeer(peerID, func(topicRPC *pb.TopicRpc) {
		if topicRPC == nil {
			return
		}
		// Set the topic ID
		topicRPC.Topicid = proto.String(t.topicName)

		var rpc pb.RPC
		rpc.Rpcs = append(rpc.Rpcs, topicRPC)

		sendRPC(&rpc, sender)
	})
	log.Debugf("peer %s added to topic %s (total peers: %d)", peerID, t.topicName, len(t.peers))
}

// removePeer removes a peer from this topic
func (t *Topic) removePeer(peerID peer.ID) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	delete(t.peers, peerID)
	t.router.RemovePeer(peerID)
	log.Debugf("peer %s removed from topic %s (total peers: %d)", peerID, t.topicName, len(t.peers))
}

// hasPeer checks if a peer is connected to this topic
func (t *Topic) hasPeer(peerID peer.ID) bool {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	_, exists := t.peers[peerID]
	return exists
}

// handleIncomingRPC forwards RPC messages to the router
func (t *Topic) handleIncomingRPC(fromPeer peer.ID, rpc *pb.TopicRpc) {
	t.router.HandleIncomingRPC(fromPeer, rpc)
}

// background processes incoming messages from the router
func (t *Topic) background() {
	defer t.wg.Done()
	if t.router == nil {
		return
	}

	for {
		// Get next message from router
		message, err := t.router.Next(t.ctx)
		if err != nil {
			return
		}
		// Deliver to all subscriptions
		t.mutex.Lock()
		for subscription := range t.subscriptions {
			subscription.put(message)
		}
		t.mutex.Unlock()
	}
}
