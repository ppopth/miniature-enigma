package pubsub

import (
	"context"
	"fmt"
	"sync"

	"github.com/ppopth/go-libp2p-cat/host"

	"github.com/libp2p/go-libp2p/core/peer"
)

// Topic is the handle for a pubsub topic
type Topic struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	lk sync.Mutex

	topic string

	// the set of subscriptions this topic has
	mySubs map[*Subscription]struct{}
	peers  map[peer.ID]host.DgramConnection
	lns    []TopicEventListener
}

func newTopic(topic string) *Topic {
	ctx, cancel := context.WithCancel(context.Background())
	return &Topic{
		ctx:    ctx,
		cancel: cancel,

		topic:  topic,
		mySubs: make(map[*Subscription]struct{}),
		peers:  make(map[peer.ID]host.DgramConnection),
	}
}

// String returns the topic associated with topic.
func (t *Topic) String() string {
	return t.topic
}

func (t *Topic) Close() error {
	t.cancel()
	t.wg.Wait()
	return nil
}

// Subscribe subscribes the topic and returns a Subscription object.
func (t *Topic) Subscribe() (*Subscription, error) {
	select {
	case <-t.ctx.Done():
		return nil, fmt.Errorf("the topic has been closed")
	default:
	}

	sub := &Subscription{t: t}

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

func (t *Topic) addPeer(p peer.ID, conn host.DgramConnection) {
	t.peers[p] = conn
}

func (t *Topic) removePeer(p peer.ID) {
	delete(t.peers, p)
}

func (t *Topic) hasPeer(p peer.ID) bool {
	_, ok := t.peers[p]
	return ok
}
