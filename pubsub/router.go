package pubsub

import (
	"context"

	"github.com/ppopth/p2p-broadcast/pb"

	"github.com/libp2p/go-libp2p/core/peer"
)

// Router is a pubsub algorithm used in each topic.
type Router interface {
	Publish([]byte)
	Next(context.Context) ([]byte, error)

	AddPeer(peer.ID, TopicSendFunc)
	RemovePeer(peer.ID)

	HandleIncomingRPC(peer.ID, *pb.TopicRpc)

	Close() error
}
