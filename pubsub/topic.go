package pubsub

import (
	"sync"
)

// Topic is the handle for a pubsub topic
type Topic struct {
	lk sync.Mutex

	p     *PubSub
	topic string

	// the set of subscriptions this topic has
	mySubs map[*Subscription]struct{}
	lns    []TopicEventListener
}

func newTopic(p *PubSub, topic string) *Topic {
	return &Topic{
		p:      p,
		topic:  topic,
		mySubs: make(map[*Subscription]struct{}),
	}
}

// String returns the topic associated with topic.
func (t *Topic) String() string {
	return t.topic
}

// Subscribe subscribes the topic and returns a Subscription object.
func (t *Topic) Subscribe() (*Subscription, error) {
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
	for _, ln := range t.lns {
		ln(ev)
	}
}
