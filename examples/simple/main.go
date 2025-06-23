package main

import (
	"context"
	"flag"
	"net"
	"net/netip"

	"github.com/ppopth/go-libp2p-cat/host"
	"github.com/ppopth/go-libp2p-cat/pubsub"
)

var (
	listenFlag  = flag.Uint("l", 0, "the listening port")
	connectFlag = flag.String("c", "", "the remote address and port")
)

func main() {
	flag.Parse()

	var opts []host.HostOption
	if *listenFlag != 0 {
		opts = append(opts, host.WithAddrPort(
			netip.AddrPortFrom(netip.IPv4Unspecified(), uint16(*listenFlag)),
		))
	}

	ctx := context.Background()
	h, err := host.NewHost(ctx, opts...)
	if err != nil {
		panic(err)
	}
	ps, err := pubsub.NewPubSub(ctx, h)
	if err != nil {
		panic(err)
	}
	topic, err := ps.Join("mytopic")
	if err != nil {
		panic(err)
	}
	sub, err := topic.Subscribe()
	if err != nil {
		panic(err)
	}

	if *connectFlag != "" {
		addr, err := net.ResolveUDPAddr("udp", *connectFlag)
		if err != nil {
			panic(err)
		}
		if err = h.Connect(ctx, addr); err != nil {
			panic(err)
		}
	}

	// TODO
	_ = sub
	for {
	}
}
