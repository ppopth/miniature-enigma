package pubsub

import (
	"context"
	"fmt"
	"sync"
)

type Subscription struct {
	ctx    context.Context
	cancel context.CancelFunc

	lk   sync.Mutex
	cond *sync.Cond

	t        *Topic
	received [][]byte
}

func newSubscription(t *Topic) *Subscription {
	ctx, cancel := context.WithCancel(context.Background())
	sub := &Subscription{
		ctx:    ctx,
		cancel: cancel,
		t:      t,
	}
	sub.cond = sync.NewCond(&sub.lk)
	return sub
}

// Next returns the next message in our subscription
func (sub *Subscription) Next(ctx context.Context) ([]byte, error) {
	sub.lk.Lock()
	defer sub.lk.Unlock()

	if len(sub.received) > 0 {
		buf := sub.received[0]
		sub.received = sub.received[1:]
		return buf, nil
	}

	unregisterAfterFunc := context.AfterFunc(ctx, func() {
		// Wake up all the waiting routines. The only routine that correponds
		// to this Next call will return from the function. Note that this can
		// be expensive, if there are too many waiting Wait calls.
		sub.cond.Broadcast()
	})
	defer unregisterAfterFunc()

	for len(sub.received) == 0 {
		sub.cond.Wait()
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("the call has been cancelled")
		case <-sub.ctx.Done():
			return nil, fmt.Errorf("the subscription has been closed")
		default:
		}
	}
	buf := sub.received[0]
	sub.received = sub.received[1:]
	return buf, nil
}

func (sub *Subscription) put(buf []byte) {
	sub.lk.Lock()
	defer sub.lk.Unlock()

	sub.received = append(sub.received, buf)
	sub.cond.Signal()
}

func (sub *Subscription) Close() error {
	sub.cancel()
	sub.cond.Broadcast()
	return nil
}
