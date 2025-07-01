package pubsub

import (
	"context"
	"fmt"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/ppopth/p2p-broadcast/host"
	"github.com/ppopth/p2p-broadcast/pb"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/peer"
)

var log = logging.Logger("pubsub")

type Option func(*PubSub) error

// NewPubSub returns a new PubSub management object.
func NewPubSub(h *host.Host, opts ...Option) (*PubSub, error) {
	ctx, cancel := context.WithCancel(context.Background())

	ps := &PubSub{
		ctx:    ctx,
		cancel: cancel,

		host:     h,
		peers:    make(map[peer.ID]host.Connection),
		topics:   make(map[string]map[peer.ID]struct{}),
		myTopics: make(map[string]*Topic),
		mySubs:   make(map[string]struct{}),
	}

	for _, opt := range opts {
		err := opt(ps)
		if err != nil {
			return nil, err
		}
	}

	h.SetPeerHandlers(ps.handleAddPeer, ps.handleRemovePeer)

	return ps, nil
}

// Join joins the topic and returns a Topic handle. Only one Topic handle should exist per topic, and Join will error if
// the Topic handle already exists.
func (p *PubSub) Join(topic string, rt Router) (*Topic, error) {
	p.lk.Lock()
	defer p.lk.Unlock()

	select {
	case <-p.ctx.Done():
		return nil, fmt.Errorf("the pubsub has been closed")
	default:
	}

	_, ok := p.myTopics[topic]
	if ok {
		return nil, fmt.Errorf("topic already exists")
	}
	t := newTopic(topic, rt)
	p.myTopics[topic] = t

	t.AddEventListener(func(ev TopicEvent) {
		p.handleTopicEvent(topic, ev)
	})

	for pid := range p.topics[topic] {
		t.addPeer(pid, p.peers[pid])
	}
	return t, nil
}

func (p *PubSub) Close() error {
	var myTopics []*Topic
	// Cancel first to prevent new joined topics
	p.cancel()
	// Copy the topics first because we cannot Close with the lock acquired
	p.lk.Lock()
	for _, t := range p.myTopics {
		myTopics = append(myTopics, t)
	}
	p.lk.Unlock()
	for _, t := range myTopics {
		t.Close()
	}
	p.wg.Wait()
	return nil
}

// sendHelloPacket sends the initial RPC containing all of our subscriptions to send to new peers
func (p *PubSub) sendHelloPacket(conn host.Sender) {
	var rpc pb.RPC

	subscriptions := make(map[string]bool)

	p.lk.Lock()
	for t := range p.mySubs {
		subscriptions[t] = true
	}
	p.lk.Unlock()

	for t := range subscriptions {
		as := &pb.RPC_SubOpts{
			Topicid:   proto.String(t),
			Subscribe: proto.Bool(true),
		}
		rpc.Subscriptions = append(rpc.Subscriptions, as)
	}

	sendRPC(&rpc, conn)
}

func (p *PubSub) handleAddPeer(pid peer.ID, conn host.Connection) {
	// Event loop to read messages from the connections
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			msgbytes, err := conn.Receive(p.ctx)
			if err != nil {
				// Quietly return
				return
			}

			rpc := &pb.RPC{}
			err = rpc.Unmarshal(msgbytes)
			if err != nil {
				log.Warnf("invalid packet received: %v", err)
				continue
			}
			p.handleIncomingRPC(pid, rpc)
		}
	}()

	p.lk.Lock()
	p.peers[pid] = conn
	p.lk.Unlock()

	// Send the initial packet for a new connection
	p.sendHelloPacket(conn)
}
func (p *PubSub) handleRemovePeer(pid peer.ID) {
	p.lk.Lock()
	defer p.lk.Unlock()

	delete(p.peers, pid)
	for _, t := range p.myTopics {
		if t.hasPeer(pid) {
			t.removePeer(pid)
		}
	}
	for _, tmap := range p.topics {
		if _, ok := tmap[pid]; ok {
			delete(tmap, pid)
		}
	}
}

func (p *PubSub) handleTopicEvent(topic string, ev TopicEvent) {
	p.lk.Lock()
	defer p.lk.Unlock()

	var subscribe bool
	var isSubscribeEvent bool

	switch ev {
	case TopicEventSubscribe:
		p.mySubs[topic] = struct{}{}
		isSubscribeEvent = true
		subscribe = true
	case TopicEventUnsubscribe:
		delete(p.mySubs, topic)
		isSubscribeEvent = true
		subscribe = false
	}

	if isSubscribeEvent {
		for _, conn := range p.peers {
			var rpc pb.RPC
			as := &pb.RPC_SubOpts{
				Topicid:   proto.String(topic),
				Subscribe: proto.Bool(subscribe),
			}
			rpc.Subscriptions = append(rpc.Subscriptions, as)
			sendRPC(&rpc, conn)
		}
	}
}

// handleIncomingRPC handles the received RPC
func (p *PubSub) handleIncomingRPC(from peer.ID, rpc *pb.RPC) {
	log.Debugf("received an RPC from %s: %v", from, rpc)
	subs := rpc.GetSubscriptions()

	if len(subs) > 0 {
		p.handleSubscriptions(from, subs)
	}

	for _, trpc := range rpc.GetRpcs() {
		if t, ok := p.myTopics[trpc.GetTopicid()]; ok {
			t.handleIncomingRPC(from, trpc)
		}
	}
}

// handleSubscriptions handles all the subscription detail in the RPC
func (p *PubSub) handleSubscriptions(from peer.ID, subs []*pb.RPC_SubOpts) {
	p.lk.Lock()
	defer p.lk.Unlock()
	for _, subopt := range subs {
		topic := subopt.GetTopicid()

		if subopt.GetSubscribe() {
			tmap, ok := p.topics[topic]
			if !ok {
				tmap = make(map[peer.ID]struct{})
				p.topics[topic] = tmap
			}

			if _, ok = tmap[from]; !ok {
				tmap[from] = struct{}{}
				// If the topic is our joined topic, notify the topic about the new peer
				if t, ok := p.myTopics[topic]; ok {
					t.addPeer(from, p.peers[from])
				}
			}
		} else {
			tmap, ok := p.topics[topic]
			if !ok {
				continue
			}

			if _, ok := tmap[from]; ok {
				delete(tmap, from)
				// If the topic is our joined topic, notify the topic about the removed peer
				if t, ok := p.myTopics[topic]; ok {
					t.removePeer(from)
				}
			}
		}
	}
}

// PubSub is the implementation of the pubsub system.
type PubSub struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	lk sync.Mutex

	host *host.Host

	// peers tracks all the peer connections
	peers map[peer.ID]host.Connection
	// topics tracks which topics each of our peers are subscribed to
	topics map[string]map[peer.ID]struct{}
	// the set of topics we are interested in
	myTopics map[string]*Topic
	// the set of topics we are subscribed to
	mySubs map[string]struct{}
}

// sendRPC sends an RPC to the connection
func sendRPC(rpc *pb.RPC, conn host.Sender) {
	log.Debugf("sent an RPC to %s: %v", conn.RemoteAddr(), rpc)

	buf, err := rpc.Marshal()
	if err != nil {
		log.Errorf("error marshalling an RPC: %v", err)
		return
	}

	if err := conn.Send(buf); err != nil {
		log.Errorf("error sending an RPC to %s: %v", conn.RemoteAddr(), err)
		return
	}
}
