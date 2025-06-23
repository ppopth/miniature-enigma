package pubsub

import (
	"context"
	"fmt"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/ppopth/go-libp2p-cat/host"
	"github.com/ppopth/go-libp2p-cat/pb"

	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p/core/peer"
)

var log = logging.Logger("pubsub")

type Option func(*PubSub) error

// NewPubSub returns a new PubSub management object.
func NewPubSub(ctx context.Context, h *host.Host, opts ...Option) (*PubSub, error) {
	ps := &PubSub{
		ctx:      ctx,
		host:     h,
		peers:    make(map[peer.ID]host.DgramConnection),
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
func (p *PubSub) Join(topic string) (*Topic, error) {
	p.lk.Lock()
	defer p.lk.Unlock()

	_, ok := p.myTopics[topic]
	if ok {
		return nil, fmt.Errorf("topic already exists")
	}
	t := newTopic(p, topic)
	p.myTopics[topic] = t

	t.AddEventListener(func(ev TopicEvent) {
		p.handleTopicEvent(topic, ev)
	})
	return t, nil
}

// sendHelloPacket sends the initial RPC containing all of our subscriptions to send to new peers
func (p *PubSub) sendHelloPacket(conn host.DgramConnection) {
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

	p.sendPacket(&rpc, conn)
}

// sendPacket sends an RPC to the datagram connection
func (p *PubSub) sendPacket(rpc *pb.RPC, conn host.DgramConnection) {
	log.Debugf("sent an RPC to %s: %v", conn.RemoteAddr(), rpc)

	buf, err := rpc.Marshal()
	if err != nil {
		log.Errorf("error marshalling an RPC: %v", err)
		return
	}

	if err := conn.SendDatagram(buf); err != nil {
		log.Errorf("error sending an RPC to %s: %v", conn.RemoteAddr(), err)
		return
	}
}

func (p *PubSub) handleAddPeer(pid peer.ID, conn host.DgramConnection) {
	// Event loop to read messages from the connections
	go func() {
		for {
			msgbytes, err := conn.ReceiveDatagram(p.ctx)
			if err != nil {
				// Quietly return
				return
			}

			rpc := &pb.RPC{}
			err = rpc.Unmarshal(msgbytes)
			if err != nil {
				log.Warnf("invalid datagram received: %v", err)
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
	for topic := range p.topics {
		delete(p.topics[topic], pid)
	}
}

func (p *PubSub) handleTopicEvent(topic string, ev TopicEvent) {
	p.lk.Lock()
	defer p.lk.Unlock()

	switch ev {
	case TopicEventSubscribe:
		p.mySubs[topic] = struct{}{}
	case TopicEventUnsubscribe:
		delete(p.mySubs, topic)
	}
}

// handleIncomingRPC handles all the received RPCs
func (p *PubSub) handleIncomingRPC(from peer.ID, rpc *pb.RPC) {
	log.Debugf("received an RPC from %s: %v", from, rpc)
	subs := rpc.GetSubscriptions()

	if len(subs) > 0 {
		p.handleSubscriptions(from, subs)
	}
}

func (p *PubSub) handleSubscriptions(from peer.ID, subs []*pb.RPC_SubOpts) {
	p.lk.Lock()
	defer p.lk.Lock()
	for _, subopt := range subs {
		t := subopt.GetTopicid()

		if subopt.GetSubscribe() {
			tmap, ok := p.topics[t]
			if !ok {
				tmap = make(map[peer.ID]struct{})
				p.topics[t] = tmap
			}

			if _, ok = tmap[from]; !ok {
				tmap[from] = struct{}{}
			}
		} else {
			tmap, ok := p.topics[t]
			if !ok {
				continue
			}

			if _, ok := tmap[from]; ok {
				delete(tmap, from)
			}
		}
	}
}

// PubSub is the implementation of the pubsub system.
type PubSub struct {
	ctx context.Context
	lk  sync.Mutex

	host *host.Host

	// peers tracks all the peer connections
	peers map[peer.ID]host.DgramConnection
	// topics tracks which topics each of our peers are subscribed to
	topics map[string]map[peer.ID]struct{}
	// the set of topics we are interested in
	myTopics map[string]*Topic
	// the set of topics we are subscribed to
	mySubs map[string]struct{}
}

// PubSubRouter is the message router component of PubSub.
type PubSubRouter interface {
}
