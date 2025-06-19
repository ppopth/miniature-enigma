package cat

import (
	"context"

	pb "github.com/ppopth/go-libp2p-cat/pb"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

type Option func(*PubSub) error

// NewPubSub returns a new PubSub management object.
func NewPubSub(ctx context.Context, rt PubSubRouter, opts ...Option) (*PubSub, error) {
	ps := &PubSub{
		rt:       rt,
		incoming: make(chan *RPC, 32),
		topics:   make(map[string]map[peer.ID]struct{}),
	}

	for _, opt := range opts {
		err := opt(ps)
		if err != nil {
			return nil, err
		}
	}

	rt.Attach(ps)

	go ps.processLoop(ctx)

	return ps, nil
}

// processLoop handles all inputs arriving on the channels
func (p *PubSub) processLoop(ctx context.Context) {
	defer func() {
		// Clean up go routines.
		p.topics = nil
	}()

	for {
		select {
		case rpc := <-p.incoming:
			p.handleIncomingRPC(rpc)
		}
	}
}

func (p *PubSub) handleIncomingRPC(rpc *RPC) {
	subs := rpc.GetSubscriptions()

	for _, subopt := range subs {
		t := subopt.GetTopicid()

		if subopt.GetSubscribe() {
			tmap, ok := p.topics[t]
			if !ok {
				tmap = make(map[peer.ID]struct{})
				p.topics[t] = tmap
			}

			if _, ok = tmap[rpc.from]; !ok {
				tmap[rpc.from] = struct{}{}
			}
		} else {
			tmap, ok := p.topics[t]
			if !ok {
				continue
			}

			if _, ok := tmap[rpc.from]; ok {
				delete(tmap, rpc.from)
			}
		}
	}
}

// PubSub is the implementation of the pubsub system.
type PubSub struct {
	rt PubSubRouter

	// incoming messages from other peers
	incoming chan *RPC

	// topics tracks which topics each of our peers are subscribed to
	topics map[string]map[peer.ID]struct{}
}

// PubSubRouter is the message router component of PubSub.
type PubSubRouter interface {
	// Protocols returns the list of protocols supported by the router.
	Protocols() []protocol.ID
	// Attach is invoked by the PubSub constructor to attach the router to a
	// freshly initialized PubSub instance.
	Attach(*PubSub)
	// AddPeer notifies the router that a new peer has been connected.
	AddPeer(peer.ID, protocol.ID)
	// RemovePeer notifies the router that a peer has been disconnected.
	RemovePeer(peer.ID)
	// Publish is invoked to forward a new message that has been validated.
	Publish(*Message)
	// Join notifies the router that we want to receive and forward messages in a topic.
	// It is invoked after the subscription announcement.
	Join(topic string)
	// Leave notifies the router that we are no longer interested in a topic.
	// It is invoked after the unsubscription announcement.
	Leave(topic string)
}

type Message struct {
}

type RPC struct {
	pb.RPC

	// unexported on purpose, not sending this over the wire
	from peer.ID
}
