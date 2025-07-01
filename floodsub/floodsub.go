package floodsub

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ppopth/p2p-broadcast/pb"
	"github.com/ppopth/p2p-broadcast/pubsub"

	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	TimeCacheDuration = 120 * time.Second
)

type MsgIdFunc func([]byte) string

type FloodSubRouter struct {
	ctx    context.Context
	cancel context.CancelFunc

	lk   sync.Mutex
	cond *sync.Cond

	peers    map[peer.ID]pubsub.TopicSendFunc
	idFunc   MsgIdFunc
	seen     *TimeCache
	received [][]byte
}

func NewFloodsub(idFunc MsgIdFunc) *FloodSubRouter {
	ctx, cancel := context.WithCancel(context.Background())
	fs := &FloodSubRouter{
		ctx:    ctx,
		cancel: cancel,

		peers:  make(map[peer.ID]pubsub.TopicSendFunc),
		idFunc: idFunc,
		seen:   NewTimeCache(TimeCacheDuration),
	}
	fs.cond = sync.NewCond(&fs.lk)
	return fs
}

func (fs *FloodSubRouter) Publish(buf []byte) {
	fs.handleMessages([][]byte{buf})
}

func (fs *FloodSubRouter) Next(ctx context.Context) ([]byte, error) {
	fs.lk.Lock()
	defer fs.lk.Unlock()

	if len(fs.received) > 0 {
		buf := fs.received[0]
		fs.received = fs.received[1:]
		return buf, nil
	}

	unregisterAfterFunc := context.AfterFunc(ctx, func() {
		// Wake up all the waiting routines. The only routine that correponds
		// to this Next call will return from the function. Note that this can
		// be expensive, if there are too many waiting Wait calls.
		fs.cond.Broadcast()
	})
	defer unregisterAfterFunc()

	for len(fs.received) == 0 {
		fs.cond.Wait()
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("the call has been cancelled")
		case <-fs.ctx.Done():
			return nil, fmt.Errorf("the router has been closed")
		default:
		}
	}
	buf := fs.received[0]
	fs.received = fs.received[1:]
	return buf, nil
}

func (fs *FloodSubRouter) AddPeer(p peer.ID, sendFunc pubsub.TopicSendFunc) {
	fs.lk.Lock()
	defer fs.lk.Unlock()

	fs.peers[p] = sendFunc
}

func (fs *FloodSubRouter) RemovePeer(p peer.ID) {
	fs.lk.Lock()
	defer fs.lk.Unlock()

	delete(fs.peers, p)
}

func (fs *FloodSubRouter) HandleIncomingRPC(from peer.ID, trpc *pb.TopicRpc) {
	fsRpc := trpc.GetFloodsub()
	fs.handleMessages(fsRpc.GetMessages())
}

func (fs *FloodSubRouter) Close() error {
	fs.cancel()
	fs.cond.Broadcast()
	fs.seen.Close()
	return nil
}

func (fs *FloodSubRouter) handleMessages(msgs [][]byte) {
	fs.lk.Lock()
	var sendFuncs []pubsub.TopicSendFunc
	for _, sendFunc := range fs.peers {
		sendFuncs = append(sendFuncs, sendFunc)
	}

	rpc := &pb.TopicRpc{
		Floodsub: &pb.FloodsubRpc{},
	}
	for _, m := range msgs {
		id := fs.idFunc(m)
		if !fs.seen.Has(id) {
			fs.seen.Add(id)
			fs.received = append(fs.received, m)
			fs.cond.Signal()
			rpc.Floodsub.Messages = append(rpc.Floodsub.Messages, m)
		}
	}

	fs.lk.Unlock()
	if len(rpc.Floodsub.Messages) > 0 {
		for _, sendFunc := range sendFuncs {
			sendFunc(rpc)
		}
	}
}
