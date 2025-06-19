package cat

import (
	"context"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"

	logging "github.com/ipfs/go-log/v2"
)

const (
	CatID = protocol.ID("/cat/1.0.0")
)

var log = logging.Logger("cat")

// NewCat returns a new PubSub object using the CatRouter.
func NewCat(ctx context.Context, opts ...Option) (*PubSub, error) {
	rt := &CatRouter{
		protocols: []protocol.ID{CatID},
	}
	return NewPubSub(ctx, rt, opts...)
}

func (c *CatRouter) Protocols() []protocol.ID {
	return c.protocols
}

func (c *CatRouter) Attach(p *PubSub) {
	c.p = p
}

func (c *CatRouter) AddPeer(p peer.ID, proto protocol.ID) {}

func (c *CatRouter) RemovePeer(p peer.ID) {}

func (c *CatRouter) Publish(*Message) {
	// TODO
}

func (c *CatRouter) Join(topic string) {}

func (c *CatRouter) Leave(topic string) {}

// CatRouter is a router that implements the CAT protocol.
type CatRouter struct {
	p         *PubSub
	protocols []protocol.ID
}
