package pubsub

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/ppopth/p2p-broadcast/host"
)

func TestHelloPacket(t *testing.T) {
	h1, err := host.NewHost(host.WithAddrPort(netip.MustParseAddrPort("127.0.0.1:0")))
	if err != nil {
		t.Fatal(err)
	}
	p1, err := NewPubSub(h1)
	if err != nil {
		t.Fatal(err)
	}

	h2, err := host.NewHost(host.WithAddrPort(netip.MustParseAddrPort("127.0.0.1:0")))
	if err != nil {
		t.Fatal(err)
	}
	p2, err := NewPubSub(h2)
	if err != nil {
		t.Fatal(err)
	}
	topics := []string{"foo", "bar"}
	for _, topic := range topics {
		tp, err := p2.Join(topic, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = tp.Subscribe()
		if err != nil {
			t.Fatal(err)
		}
	}

	addr := h1.LocalAddr()
	if err := h2.Connect(context.Background(), addr); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)

	// Check that h1 remembers the subscribed topics of h2
	for _, topic := range topics {
		tmap, ok := p1.topics[topic]
		if !ok {
			t.Fatalf("topic %s not found in p1.topics", topic)
		}
		_, ok = tmap[h2.ID()]
		if !ok {
			t.Fatalf("pid %s not found in p1.topics", h2.ID())
		}
	}

	p1.Close()
	p2.Close()

	h1.Close()
	h2.Close()
}

func TestJoinAlreadyJoinedTopic(t *testing.T) {
	h, err := host.NewHost(host.WithAddrPort(netip.MustParseAddrPort("127.0.0.1:0")))
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewPubSub(h)
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Join("foo", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Join("foo", nil)
	if err == nil {
		t.Fatal("should get an error when join an already joined topic")
	}

	p.Close()
	h.Close()
}

func TestClose(t *testing.T) {
	h1, err := host.NewHost(host.WithAddrPort(netip.MustParseAddrPort("127.0.0.1:0")))
	if err != nil {
		t.Fatal(err)
	}
	p1, err := NewPubSub(h1)
	if err != nil {
		t.Fatal(err)
	}

	// Close PubSub before Host
	p1.Close()
	h1.Close()

	h2, err := host.NewHost(host.WithAddrPort(netip.MustParseAddrPort("127.0.0.1:0")))
	if err != nil {
		t.Fatal(err)
	}
	p2, err := NewPubSub(h2)
	if err != nil {
		t.Fatal(err)
	}

	// Close Host before PubSub
	h2.Close()
	p2.Close()
}

func TestJoinOnClosed(t *testing.T) {
	h, err := host.NewHost(host.WithAddrPort(netip.MustParseAddrPort("127.0.0.1:0")))
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewPubSub(h)
	if err != nil {
		t.Fatal(err)
	}

	p.Close()

	_, err = p.Join("foo", nil)
	if err == nil {
		t.Fatal("expected error when join on a closed pubsub")
	}

	h.Close()
}
