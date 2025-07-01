package main

import (
	"context"
	"flag"
	"net"
	"net/netip"

	"github.com/ppopth/p2p-broadcast/host"
	"github.com/ppopth/p2p-broadcast/pubsub"
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

	h, err := host.NewHost(opts...)
	if err != nil {
		panic(err)
	}
	ps, err := pubsub.NewPubSub(h)
	if err != nil {
		panic(err)
	}
	topic, err := ps.Join("mytopic", nil)
	if err != nil {
		panic(err)
	}
	_, err = topic.Subscribe()
	if err != nil {
		panic(err)
	}

	if *connectFlag != "" {
		addr, err := net.ResolveUDPAddr("udp", *connectFlag)
		if err != nil {
			panic(err)
		}
		if err = h.Connect(context.Background(), addr); err != nil {
			panic(err)
		}
	}

	for {
	}
}
