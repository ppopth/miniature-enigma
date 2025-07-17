package pubsub

import (
	"context"
	"fmt"
	"sync"
)

// Subscription represents a subscription to a topic
type Subscription struct {
	ctx    context.Context
	cancel context.CancelFunc

	mutex sync.Mutex // Protects received messages
	cond  *sync.Cond // Notifies waiting consumers

	topic            *Topic   // Parent topic
	receivedMessages [][]byte // Buffered messages
}

// newSubscription creates a new subscription to a topic
func newSubscription(topic *Topic) *Subscription {
	ctx, cancel := context.WithCancel(context.Background())
	subscription := &Subscription{
		ctx:    ctx,
		cancel: cancel,
		topic:  topic,
	}
	subscription.cond = sync.NewCond(&subscription.mutex)
	return subscription
}

// Next returns the next message from the subscription
func (sub *Subscription) Next(ctx context.Context) ([]byte, error) {
	sub.mutex.Lock()
	defer sub.mutex.Unlock()

	// Return immediately if we have buffered messages
	if len(sub.receivedMessages) > 0 {
		message := sub.receivedMessages[0]
		sub.receivedMessages = sub.receivedMessages[1:]
		return message, nil
	}

	// Set up context cancellation to wake up waiters
	unregisterAfterFunc := context.AfterFunc(ctx, func() {
		// Wake up all waiting routines when context is cancelled
		sub.cond.Broadcast()
	})
	defer unregisterAfterFunc()

	// Wait for messages to arrive
	for len(sub.receivedMessages) == 0 {
		sub.cond.Wait()
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled")
		case <-sub.ctx.Done():
			return nil, fmt.Errorf("subscription closed")
		default:
		}
	}
	message := sub.receivedMessages[0]
	sub.receivedMessages = sub.receivedMessages[1:]
	return message, nil
}

// put adds a message to the subscription buffer
func (sub *Subscription) put(message []byte) {
	sub.mutex.Lock()
	defer sub.mutex.Unlock()

	sub.receivedMessages = append(sub.receivedMessages, message)
	sub.cond.Signal()
}

// Close closes the subscription
func (sub *Subscription) Close() error {
	sub.cancel()
	sub.cond.Broadcast()
	// Remove this subscription from the topic
	sub.topic.removeSubscription(sub)
	return nil
}
