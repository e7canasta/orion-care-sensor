// Package framebus provides non-blocking frame distribution to multiple subscribers.
//
// FrameBus implements a fan-out pattern where frames published to the bus are distributed
// to all registered subscribers using Go channels. If a subscriber's channel is full,
// the frame is dropped rather than queued, maintaining real-time processing semantics.
//
// # Core Philosophy
//
// "Drop frames, never queue. Latency > Completeness."
//
// FrameBus prioritizes low latency over guaranteed delivery. This design choice is
// intentional for real-time video processing where processing recent frames is more
// valuable than processing a backlog of stale frames.
//
// # Basic Usage
//
//	bus := framebus.New()
//	defer bus.Close()
//
//	// Create subscriber channel
//	workerCh := make(chan framebus.Frame, 5)
//	bus.Subscribe("worker-1", workerCh)
//
//	// Publish frames (non-blocking)
//	for frame := range source {
//	    bus.Publish(frame)
//	}
//
//	// Check stats
//	stats := bus.Stats()
//	fmt.Printf("Published: %d, Sent: %d, Dropped: %d\n",
//	    stats.TotalPublished, stats.TotalSent, stats.TotalDropped)
//
// # Thread Safety
//
// All methods are safe for concurrent use. Multiple goroutines can call Publish()
// simultaneously, and Subscribe/Unsubscribe can be called while publishing.
//
// # Performance
//
// Publish() completes in microseconds and never blocks, even with slow subscribers.
// Memory usage is constant (bounded by subscriber channel buffers).
package framebus

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// Bus distributes frames to multiple subscribers with drop policy.
type Bus interface {
	// Subscribe registers a channel to receive frames.
	// Returns error if id already exists or if bus is closed.
	Subscribe(id string, ch chan<- Frame) error

	// Unsubscribe removes a subscriber by id.
	// Returns error if id not found or if bus is closed.
	Unsubscribe(id string) error

	// Publish sends frame to all subscribers (non-blocking).
	// Drops frame for subscribers whose channels are full.
	// Panics if bus is closed.
	Publish(frame Frame)

	// Stats returns current bus statistics snapshot.
	Stats() BusStats

	// Close stops the bus and prevents further operations.
	// Subsequent Subscribe/Unsubscribe will return ErrBusClosed.
	// Subsequent Publish will panic.
	Close() error
}

var (
	// ErrSubscriberExists is returned when Subscribe is called with a duplicate id.
	ErrSubscriberExists = errors.New("subscriber id already exists")

	// ErrSubscriberNotFound is returned when Unsubscribe is called with unknown id.
	ErrSubscriberNotFound = errors.New("subscriber id not found")

	// ErrBusClosed is returned when operations are attempted on a closed bus.
	ErrBusClosed = errors.New("bus is closed")
)

// Frame represents a video frame to be distributed.
type Frame struct {
	// Data contains raw frame data (JPEG, PNG, etc.)
	Data []byte

	// Seq is the frame sequence number
	Seq uint64

	// Timestamp is when the frame was captured
	Timestamp time.Time

	// Metadata contains optional key-value pairs
	Metadata map[string]string
}

// BusStats contains global and per-subscriber metrics.
type BusStats struct {
	// TotalPublished is the number of Publish() calls
	TotalPublished uint64

	// TotalSent is the sum of frames sent to all subscribers
	TotalSent uint64

	// TotalDropped is the sum of frames dropped across all subscribers
	TotalDropped uint64

	// Subscribers contains per-subscriber breakdown
	Subscribers map[string]SubscriberStats
}

// SubscriberStats tracks metrics for a single subscriber.
type SubscriberStats struct {
	// Sent is the number of frames successfully sent to this subscriber
	Sent uint64

	// Dropped is the number of frames dropped due to full channel
	Dropped uint64
}

// subscriberStats holds internal atomic counters.
type subscriberStats struct {
	sent    atomic.Uint64
	dropped atomic.Uint64
}

// bus is the concrete implementation of Bus.
type bus struct {
	mu          sync.RWMutex
	subscribers map[string]chan<- Frame
	stats       map[string]*subscriberStats
	closed      bool

	// Global counter (atomic - no lock needed in Publish)
	totalPublished atomic.Uint64
}

// New creates a new FrameBus.
func New() Bus {
	return &bus{
		subscribers: make(map[string]chan<- Frame),
		stats:       make(map[string]*subscriberStats),
		closed:      false,
	}
}

// Subscribe registers a channel to receive frames.
func (b *bus) Subscribe(id string, ch chan<- Frame) error {
	if ch == nil {
		return errors.New("subscriber channel cannot be nil")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return ErrBusClosed
	}

	if _, exists := b.subscribers[id]; exists {
		return ErrSubscriberExists
	}

	b.subscribers[id] = ch
	b.stats[id] = &subscriberStats{}

	return nil
}

// Unsubscribe removes a subscriber by id.
func (b *bus) Unsubscribe(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return ErrBusClosed
	}

	if _, exists := b.subscribers[id]; !exists {
		return ErrSubscriberNotFound
	}

	delete(b.subscribers, id)
	delete(b.stats, id)

	return nil
}

// Publish sends frame to all subscribers (non-blocking).
//
// For each subscriber:
//   - If channel has space: frame is sent, Sent counter incremented
//   - If channel is full: frame is dropped, Dropped counter incremented
//
// This method never blocks, even if all subscribers are slow.
// Panics if bus is closed (check with defer/recover if needed).
func (b *bus) Publish(frame Frame) {
	b.totalPublished.Add(1)

	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		panic("publish on closed bus")
	}

	for id, ch := range b.subscribers {
		select {
		case ch <- frame:
			// Frame sent successfully
			b.stats[id].sent.Add(1)
		default:
			// Channel full - drop frame
			b.stats[id].dropped.Add(1)
		}
	}
}

// Stats returns current bus statistics snapshot.
//
// The returned BusStats is a snapshot at the time of the call.
// Concurrent Publish operations may increment counters after Stats() returns.
//
// Stats() can be called concurrently with all other operations.
func (b *bus) Stats() BusStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := BusStats{
		TotalPublished: b.totalPublished.Load(),
		Subscribers:    make(map[string]SubscriberStats),
	}

	var totalSent, totalDropped uint64

	for id, stats := range b.stats {
		sent := stats.sent.Load()
		dropped := stats.dropped.Load()

		totalSent += sent
		totalDropped += dropped

		result.Subscribers[id] = SubscriberStats{
			Sent:    sent,
			Dropped: dropped,
		}
	}

	result.TotalSent = totalSent
	result.TotalDropped = totalDropped

	return result
}

// Close stops the bus and prevents further operations.
//
// After Close:
//   - Subscribe/Unsubscribe return ErrBusClosed
//   - Publish panics
//   - Stats continues to work (returns final snapshot)
//
// Close does NOT close subscriber channels - that is the subscriber's responsibility.
// Close is idempotent (safe to call multiple times).
func (b *bus) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return nil // Already closed, idempotent
	}

	b.closed = true

	// Note: We do NOT close subscriber channels
	// Each subscriber is responsible for managing their own channel lifecycle

	return nil
}
