package pubsub

import (
	"testing"
)

func TestSubscribe(t *testing.T) {
	var err error

	topic := "foo"
	tp := newTopic(topic)
	if tp.String() != topic {
		t.Fatalf("unexpected string value of the topic expected: %s actual: %s", topic, tp.String())
	}

	// Add the event listener
	var ev *TopicEvent
	listener := func(e TopicEvent) {
		if ev != nil {
			t.Fatal("the event listener was called many times")
		}
		ev = &e
	}
	tp.AddEventListener(listener)

	// First subscription
	_, err = tp.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	// After the first subscription, we should get an event
	if *ev != TopicEventSubscribe {
		t.Fatalf("expected a topic event TopicEventSubscribe; found %v", ev)
	}

	// Reset the event
	ev = nil
	// Second subscription
	_, err = tp.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	// After the second subscription, we should get no event
	if ev != nil {
		t.Fatalf("expected no topic event; found %v", ev)
	}

	tp.Close()
}

func TestSubscribeOnClosed(t *testing.T) {
	var err error

	topic := "foo"
	tp := newTopic(topic)

	_, err = tp.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	tp.Close()
	_, err = tp.Subscribe()
	if err == nil {
		t.Fatal("expected error when subscribe on a closed topic")
	}
}
