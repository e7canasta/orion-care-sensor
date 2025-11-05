package bus

import (
	"sync"
	"sync/atomic"
)

type subscriberHolder struct {
	id     string
	policy DropPolicy
	stats  *SubscriberStats

	// For DropNew policy
	ch chan<- Frame

	// For DropOld policy
	holder *latestFrameHolder
}

type bus struct {
	mu             sync.RWMutex
	subscribers    map[string]*subscriberHolder
	totalPublished uint64
	closed         bool
}

// New creates a new FrameBus instance
func New() Bus {
	return &bus{
		subscribers: make(map[string]*subscriberHolder),
	}
}

// Subscribe registers a channel with DropNew policy (current behavior)
func (b *bus) Subscribe(id string, ch chan<- Frame) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return ErrBusClosed
	}

	if _, exists := b.subscribers[id]; exists {
		return ErrSubscriberExists
	}

	if ch == nil {
		return ErrNilChannel
	}

	b.subscribers[id] = &subscriberHolder{
		id:     id,
		policy: DropNew,
		stats:  &SubscriberStats{},
		ch:     ch,
	}

	return nil
}

// SubscribeDropOld registers a subscriber with DropOld policy
func (b *bus) SubscribeDropOld(id string) (FrameReceiver, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil, ErrBusClosed
	}

	if _, exists := b.subscribers[id]; exists {
		return nil, ErrSubscriberExists
	}

	holder := &subscriberHolder{
		id:     id,
		policy: DropOld,
		stats:  &SubscriberStats{},
		holder: newLatestFrameHolder(),
	}

	b.subscribers[id] = holder
	return holder.holder, nil
}

// Publish distributes frame to all subscribers
func (b *bus) Publish(frame Frame) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return
	}

	atomic.AddUint64(&b.totalPublished, 1)

	for _, holder := range b.subscribers {
		switch holder.policy {
		case DropNew:
			// Non-blocking send to channel
			select {
			case holder.ch <- frame:
				atomic.AddUint64(&holder.stats.Sent, 1)
			default:
				atomic.AddUint64(&holder.stats.Dropped, 1)
			}

		case DropOld:
			// Replace latest frame (always succeeds)
			_ = holder.holder.Set(frame)
			atomic.AddUint64(&holder.stats.Sent, 1)
		}
	}
}

// Unsubscribe removes a subscriber
func (b *bus) Unsubscribe(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	holder, exists := b.subscribers[id]
	if !exists {
		return ErrSubscriberNotFound
	}

	// Clean up DropOld holder
	if holder.policy == DropOld && holder.holder != nil {
		holder.holder.Close()
	}

	delete(b.subscribers, id)
	return nil
}

// Stats returns statistics for a subscriber
func (b *bus) Stats(id string) (*SubscriberStats, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	holder, exists := b.subscribers[id]
	if !exists {
		return nil, ErrSubscriberNotFound
	}

	return &SubscriberStats{
		Sent:    atomic.LoadUint64(&holder.stats.Sent),
		Dropped: atomic.LoadUint64(&holder.stats.Dropped),
	}, nil
}

// Close shuts down the bus and all subscribers
func (b *bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}

	b.closed = true

	// Clean up all DropOld holders
	for _, holder := range b.subscribers {
		if holder.policy == DropOld && holder.holder != nil {
			holder.holder.Close()
		}
	}

	b.subscribers = nil
}

// latestFrameHolder implements FrameReceiver for DropOld policy
type latestFrameHolder struct {
	mu     sync.RWMutex
	cond   *sync.Cond
	frame  *Frame
	seq    uint64
	closed bool
}

func newLatestFrameHolder() *latestFrameHolder {
	h := &latestFrameHolder{}
	h.cond = sync.NewCond(&h.mu)
	return h
}

func (h *latestFrameHolder) Set(frame Frame) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return ErrReceiverClosed
	}

	h.frame = &frame
	h.seq++
	h.cond.Broadcast()
	return nil
}

// Receive blocks until a frame is available
func (h *latestFrameHolder) Receive() Frame {
	h.mu.Lock()
	defer h.mu.Unlock()

	for h.frame == nil && !h.closed {
		h.cond.Wait()
	}

	if h.closed {
		return Frame{}
	}

	return *h.frame
}

// TryReceive returns the latest frame without blocking
func (h *latestFrameHolder) TryReceive() (Frame, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.frame == nil {
		return Frame{}, false
	}

	return *h.frame, true
}

// Close shuts down the receiver
func (h *latestFrameHolder) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.closed = true
	h.cond.Broadcast()
}
