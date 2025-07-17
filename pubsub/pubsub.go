package pubsub

import (
	"context"
	"fmt"
	"sync"

	"github.com/ethp2p/eth-ec-broadcast/host"
	"github.com/ethp2p/eth-ec-broadcast/pb"
	"github.com/gogo/protobuf/proto"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/peer"
)

var log = logging.Logger("pubsub")

type Option func(*PubSub) error

// NewPubSub creates a new publish-subscribe system
func NewPubSub(hostInstance *host.Host, opts ...Option) (*PubSub, error) {
	ctx, cancel := context.WithCancel(context.Background())

	pubsub := &PubSub{
		ctx:    ctx,
		cancel: cancel,

		host:               hostInstance,
		peerConnections:    make(map[peer.ID]host.Connection),
		topicSubscriptions: make(map[string]map[peer.ID]struct{}),
		joinedTopics:       make(map[string]*Topic),
		mySubscriptions:    make(map[string]struct{}),
	}

	for _, opt := range opts {
		err := opt(pubsub)
		if err != nil {
			return nil, err
		}
	}

	hostInstance.SetPeerHandlers(pubsub.handleAddPeer, pubsub.handleRemovePeer)

	return pubsub, nil
}

// Join joins the topic and returns a Topic handle. Only one Topic handle should exist per topic, and Join will error if
// the Topic handle already exists.
func (p *PubSub) Join(topic string, rt Router) (*Topic, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	select {
	case <-p.ctx.Done():
		return nil, fmt.Errorf("the pubsub has been closed")
	default:
	}

	_, ok := p.joinedTopics[topic]
	if ok {
		return nil, fmt.Errorf("topic already exists")
	}
	t := newTopic(topic, rt)
	p.joinedTopics[topic] = t

	t.AddEventListener(func(ev TopicEvent) {
		p.handleTopicEvent(topic, ev)
	})

	for pid := range p.topicSubscriptions[topic] {
		t.addPeer(pid, p.peerConnections[pid])
	}
	return t, nil
}

// Close shuts down the pubsub system and all topics
func (p *PubSub) Close() error {
	var topicsToClose []*Topic
	// Cancel context to prevent new operations
	p.cancel()
	// Copy topics to close them without holding the lock
	p.mutex.Lock()
	for _, topic := range p.joinedTopics {
		topicsToClose = append(topicsToClose, topic)
	}
	p.mutex.Unlock()
	for _, topic := range topicsToClose {
		topic.Close()
	}
	p.wg.Wait()
	return nil
}

// sendHelloPacket sends initial subscriptions to a new peer
func (p *PubSub) sendHelloPacket(conn host.Sender) {
	var rpc pb.RPC

	subscriptions := make(map[string]bool)

	p.mutex.Lock()
	for t := range p.mySubscriptions {
		subscriptions[t] = true
	}
	p.mutex.Unlock()

	for topicName := range subscriptions {
		subOpt := &pb.RPC_SubOpts{
			Topicid:   proto.String(topicName),
			Subscribe: proto.Bool(true),
		}
		rpc.Subscriptions = append(rpc.Subscriptions, subOpt)
	}

	sendRPC(&rpc, conn)
}

// handleAddPeer is called when a new peer connects
func (p *PubSub) handleAddPeer(peerID peer.ID, conn host.Connection) {
	// Start message processing loop for this peer
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			messageBytes, err := conn.Receive(p.ctx)
			if err != nil {
				// Connection closed or context cancelled
				return
			}

			rpc := &pb.RPC{}
			err = rpc.Unmarshal(messageBytes)
			if err != nil {
				log.Warnf("invalid packet received: %v", err)
				continue
			}
			p.handleIncomingRPC(peerID, rpc)
		}
	}()

	// Register the new peer
	p.mutex.Lock()
	p.peerConnections[peerID] = conn
	p.mutex.Unlock()

	// Send our current subscriptions to the new peer
	p.sendHelloPacket(conn)
}

// handleRemovePeer is called when a peer disconnects
func (p *PubSub) handleRemovePeer(peerID peer.ID) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Remove peer from all data structures
	delete(p.peerConnections, peerID)
	for _, topic := range p.joinedTopics {
		if topic.hasPeer(peerID) {
			topic.removePeer(peerID)
		}
	}
	for _, subscriberMap := range p.topicSubscriptions {
		if _, subscribed := subscriberMap[peerID]; subscribed {
			delete(subscriberMap, peerID)
		}
	}
}

// handleTopicEvent processes subscription events from topics
func (p *PubSub) handleTopicEvent(topicName string, event TopicEvent) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	var subscribe bool
	var isSubscribeEvent bool

	switch event {
	case TopicEventSubscribe:
		p.mySubscriptions[topicName] = struct{}{}
		isSubscribeEvent = true
		subscribe = true
	case TopicEventUnsubscribe:
		delete(p.mySubscriptions, topicName)
		isSubscribeEvent = true
		subscribe = false
	}

	// Notify all peers about subscription changes
	if isSubscribeEvent {
		for _, conn := range p.peerConnections {
			var rpc pb.RPC
			subOpt := &pb.RPC_SubOpts{
				Topicid:   proto.String(topicName),
				Subscribe: proto.Bool(subscribe),
			}
			rpc.Subscriptions = append(rpc.Subscriptions, subOpt)
			sendRPC(&rpc, conn)
		}
	}
}

// handleIncomingRPC processes RPC messages from peers
func (p *PubSub) handleIncomingRPC(fromPeer peer.ID, rpc *pb.RPC) {
	log.Debugf("received RPC from %s: %v", fromPeer, rpc)
	subscriptions := rpc.GetSubscriptions()

	// Handle subscription changes
	if len(subscriptions) > 0 {
		p.handleSubscriptions(fromPeer, subscriptions)
	}

	// Forward topic-specific messages to appropriate topics
	for _, topicRPC := range rpc.GetRpcs() {
		if topic, exists := p.joinedTopics[topicRPC.GetTopicid()]; exists {
			topic.handleIncomingRPC(fromPeer, topicRPC)
		}
	}
}

// handleSubscriptions processes subscription updates from a peer
func (p *PubSub) handleSubscriptions(fromPeer peer.ID, subscriptions []*pb.RPC_SubOpts) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	for _, subOpt := range subscriptions {
		topicName := subOpt.GetTopicid()

		if subOpt.GetSubscribe() {
			// Peer is subscribing to topic
			subscriberMap, exists := p.topicSubscriptions[topicName]
			if !exists {
				subscriberMap = make(map[peer.ID]struct{})
				p.topicSubscriptions[topicName] = subscriberMap
			}

			if _, alreadySubscribed := subscriberMap[fromPeer]; !alreadySubscribed {
				subscriberMap[fromPeer] = struct{}{}
				// Notify our topic if we're participating in it
				if topic, joined := p.joinedTopics[topicName]; joined {
					topic.addPeer(fromPeer, p.peerConnections[fromPeer])
				}
			}
		} else {
			// Peer is unsubscribing from topic
			subscriberMap, exists := p.topicSubscriptions[topicName]
			if !exists {
				continue
			}

			if _, subscribed := subscriberMap[fromPeer]; subscribed {
				delete(subscriberMap, fromPeer)
				// Notify our topic if we're participating in it
				if topic, joined := p.joinedTopics[topicName]; joined {
					topic.removePeer(fromPeer)
				}
			}
		}
	}
}

// PubSub manages topics and peer subscriptions
type PubSub struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mutex sync.Mutex // Protects all maps below

	host *host.Host

	peerConnections    map[peer.ID]host.Connection     // Active peer connections
	topicSubscriptions map[string]map[peer.ID]struct{} // Which peers subscribe to each topic
	joinedTopics       map[string]*Topic               // Topics we've joined
	mySubscriptions    map[string]struct{}             // Topics we're subscribed to
}

// sendRPC marshals and sends an RPC message to a connection
func sendRPC(rpc *pb.RPC, conn host.Sender) {
	if rpc == nil || conn == nil {
		return
	}
	log.Debugf("sending RPC to %s: %v", conn.RemoteAddr(), rpc)

	// Marshal the RPC to bytes
	buffer, err := rpc.Marshal()
	if err != nil {
		log.Errorf("failed to marshal RPC: %v", err)
		return
	}

	// Send the marshaled data
	if err := conn.Send(buffer); err != nil {
		log.Errorf("failed to send RPC to %s: %v", conn.RemoteAddr(), err)
		return
	}
}
