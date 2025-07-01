package pubsub

import (
	"context"
	"fmt"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/ppopth/p2p-broadcast/host"
	"github.com/ppopth/p2p-broadcast/pb"

	"github.com/libp2p/go-libp2p/core/peer"
)

type TopicSendFunc func(*pb.TopicRpc)

// Topic is the handle for a pubsub topic
type Topic struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	lk sync.Mutex

	rt    Router
	topic string

	// the set of subscriptions this topic has
	mySubs map[*Subscription]struct{}
	peers  map[peer.ID]host.Sender
	lns    []TopicEventListener
}

func newTopic(topic string, rt Router) *Topic {
	ctx, cancel := context.WithCancel(context.Background())
	t := &Topic{
		ctx:    ctx,
		cancel: cancel,

		rt:     rt,
		topic:  topic,
		mySubs: make(map[*Subscription]struct{}),
		peers:  make(map[peer.ID]host.Sender),
	}

	t.wg.Add(1)
	go t.background()

	return t
}

// String returns the topic associated with topic.
func (t *Topic) String() string {
	return t.topic
}

func (t *Topic) Close() error {
	t.lk.Lock()
	for sub := range t.mySubs {
		sub.Close()
	}
	t.notifyEvent(TopicEventUnsubscribe)
	if t.rt != nil {
		t.rt.Close()
	}
	t.cancel()
	t.lk.Unlock()
	t.wg.Wait()
	return nil
}

func (t *Topic) Publish(buf []byte) {
	t.rt.Publish(buf)
}

// Subscribe subscribes the topic and returns a Subscription object.
func (t *Topic) Subscribe() (*Subscription, error) {
	select {
	case <-t.ctx.Done():
		return nil, fmt.Errorf("the topic has been closed")
	default:
	}

	sub := newSubscription(t)

	t.lk.Lock()
	defer t.lk.Unlock()
	if len(t.mySubs) == 0 {
		t.notifyEvent(TopicEventSubscribe)
	}
	t.mySubs[sub] = struct{}{}

	return sub, nil
}

type TopicEventListener func(TopicEvent)
type TopicEvent int

const (
	TopicEventSubscribe = iota
	TopicEventUnsubscribe
)

// AddEventListener allows others to receive events related to the topic.
func (t *Topic) AddEventListener(ln TopicEventListener) {
	select {
	case <-t.ctx.Done():
		return
	default:
	}

	t.lk.Lock()
	defer t.lk.Unlock()

	t.lns = append(t.lns, ln)
}

// SATETY: Assume that the lock has been acquired keep to the set of listeners static
func (t *Topic) notifyEvent(ev TopicEvent) {
	select {
	case <-t.ctx.Done():
		return
	default:
	}

	for _, ln := range t.lns {
		ln(ev)
	}
}

func (t *Topic) addPeer(p peer.ID, sender host.Sender) {
	t.lk.Lock()
	defer t.lk.Unlock()

	t.peers[p] = sender
	t.rt.AddPeer(p, func(trpc *pb.TopicRpc) {
		if trpc == nil {
			return
		}
		// Replace the topic id
		trpc.Topicid = proto.String(t.topic)

		var rpc pb.RPC
		rpc.Rpcs = append(rpc.Rpcs, trpc)

		sendRPC(&rpc, sender)
	})
}

func (t *Topic) removePeer(p peer.ID) {
	t.lk.Lock()
	defer t.lk.Unlock()

	delete(t.peers, p)
	t.rt.RemovePeer(p)
}

func (t *Topic) hasPeer(p peer.ID) bool {
	t.lk.Lock()
	defer t.lk.Unlock()

	_, ok := t.peers[p]
	return ok
}

func (t *Topic) handleIncomingRPC(from peer.ID, rpc *pb.TopicRpc) {
	t.rt.HandleIncomingRPC(from, rpc)
}

func (t *Topic) background() {
	defer t.wg.Done()
	if t.rt == nil {
		return
	}

	for {
		buf, err := t.rt.Next(t.ctx)
		if err != nil {
			return
		}
		t.lk.Lock()
		for sub := range t.mySubs {
			sub.put(buf)
		}
		t.lk.Unlock()
	}
}
