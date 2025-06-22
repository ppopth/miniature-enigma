package main

import (
	"context"
	"flag"
	"net"
	"net/netip"

	cat "github.com/ppopth/go-libp2p-cat"
)

var (
	listenFlag  = flag.Uint("l", 0, "the listening port")
	connectFlag = flag.String("c", "", "the remote address and port")
)

func main() {
	flag.Parse()

	var opts []cat.HostOption
	if *listenFlag != 0 {
		opts = append(opts, cat.WithAddrPort(
			netip.AddrPortFrom(netip.IPv4Unspecified(), uint16(*listenFlag)),
		))
	}

	ctx := context.Background()
	h, err := cat.NewHost(ctx, opts...)
	if err != nil {
		panic(err)
	}
	ps, err := cat.NewCat(ctx, h)
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
