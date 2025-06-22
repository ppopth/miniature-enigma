package cat

import (
	"context"
	"net/netip"
	"testing"
	"time"
)

func TestHelloPacket(t *testing.T) {
	ctx := context.Background()
	h1, err := NewHost(ctx, WithAddrPort(netip.MustParseAddrPort("127.0.0.1:0")))
	if err != nil {
		panic(err)
	}
	p1, err := NewPubSub(ctx, h1, &CatRouter{})
	if err != nil {
		panic(err)
	}

	h2, err := NewHost(ctx, WithAddrPort(netip.MustParseAddrPort("127.0.0.1:0")))
	if err != nil {
		panic(err)
	}
	p2, err := NewPubSub(ctx, h2, &CatRouter{})
	if err != nil {
		panic(err)
	}
	topics := []string{"foo", "bar"}
	for _, topic := range topics {
		tp, err := p2.Join(topic)
		if err != nil {
			panic(err)
		}
		_, err = tp.Subscribe()
		if err != nil {
			panic(err)
		}
	}

	addr := h1.tr.Conn.LocalAddr()
	if err := h2.Connect(ctx, addr); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)

	// Check that h1 remembers the subscribed topics of h2
	for _, topic := range topics {
		tmap, ok := p1.topics[topic]
		if !ok {
			t.Fatalf("topic %s not found in p1.topics", topic)
		}
		_, ok = tmap[h2.pid]
		if !ok {
			t.Fatalf("pid %s not found in p1.topics", h2.pid)
		}
	}
}
