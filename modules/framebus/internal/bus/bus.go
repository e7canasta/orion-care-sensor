// Package bus provides the internal implementation of FrameBus.
//
// This package is internal and should not be imported directly.
// Use github.com/visiona/orion/modules/framebus instead.
package bus

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
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

	// Ctx is an optional context for tracing, cancellation, and deadlines.
	// If nil, the frame has no associated context (backward compatible).
	//
	// Use cases:
	//   - Distributed tracing (OpenTelemetry trace ID propagation)
	//   - Request cancellation (cancel in-flight processing)
	//   - Deadline propagation (frame expiration time)
	//
	// Example:
	//   ctx, span := tracer.Start(context.Background(), "frame-publish")
	//   frame.Ctx = ctx
	//   bus.Publish(frame)
	//   defer span.End()
	Ctx context.Context
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

	// Priority is the subscriber's priority level
	Priority SubscriberPriority
}

// SubscriberPriority defines load shedding priority for subscribers.
//
// Lower numeric values = higher priority (protected during load shedding).
//
// Priority levels follow industry standards (AWS SQS, Kubernetes QoS):
//   - Critical: Mission-critical workloads (fall detection, medical alerts)
//   - High: Important but not life-critical (sleep quality monitoring)
//   - Normal: Standard workloads (default for backward compatibility)
//   - BestEffort: Experimental or non-essential workloads
type SubscriberPriority int

const (
	// PriorityCritical: Never drop frames if possible.
	// Use for: Mission-critical experts with SLAs (EdgeExpert, ExitExpert)
	// Drop policy: Last to drop (protected under load)
	PriorityCritical SubscriberPriority = 0

	// PriorityHigh: Drop only under severe load.
	// Use for: Important but not life-critical (SleepExpert)
	// Drop policy: Drop after BestEffort/Normal, before Critical
	PriorityHigh SubscriberPriority = 1

	// PriorityNormal: Default priority (backward compatible).
	// Use for: Standard workers without special requirements (CaregiverExpert)
	// Drop policy: Standard drop behavior
	PriorityNormal SubscriberPriority = 2

	// PriorityBestEffort: Drop first under any load.
	// Use for: Experimental models, research, telemetry (PostureExpert)
	// Drop policy: First to drop (no SLA guarantees)
	PriorityBestEffort SubscriberPriority = 3
)

// SubscriberHealth represents the health state of a subscriber based on drop rate.
type SubscriberHealth string

const (
	// HealthHealthy indicates normal operation with low drop rate (< 50%).
	HealthHealthy SubscriberHealth = "healthy"

	// HealthDegraded indicates elevated drop rate (50-90%).
	// Subscriber is falling behind but still processing some frames.
	HealthDegraded SubscriberHealth = "degraded"

	// HealthSaturated indicates critical drop rate (> 90%).
	// Subscriber is severely overloaded and requires intervention.
	HealthSaturated SubscriberHealth = "saturated"

	// HealthUnknown is returned for subscribers with no activity yet.
	HealthUnknown SubscriberHealth = "unknown"
)

var (
	// ErrSubscriberExists is returned when Subscribe is called with a duplicate id.
	ErrSubscriberExists = errors.New("subscriber id already exists")

	// ErrSubscriberNotFound is returned when Unsubscribe is called with unknown id.
	ErrSubscriberNotFound = errors.New("subscriber id not found")

	// ErrBusClosed is returned when operations are attempted on a closed bus.
	ErrBusClosed = errors.New("bus is closed")
)

// Bus distributes frames to multiple subscribers with drop policy.
type Bus interface {
	// Subscribe registers a channel to receive frames with default (Normal) priority.
	// Returns error if id already exists or if bus is closed.
	//
	// For backward compatibility, this method uses PriorityNormal.
	// Use SubscribeWithPriority() to specify a custom priority.
	Subscribe(id string, ch chan<- Frame) error

	// SubscribeWithPriority registers a channel with a specific priority level.
	// Returns error if id already exists or if bus is closed.
	//
	// Priority levels:
	//   - PriorityCritical:   Mission-critical (fall detection)
	//   - PriorityHigh:       Important (sleep monitoring)
	//   - PriorityNormal:     Standard (default)
	//   - PriorityBestEffort: Experimental (research)
	//
	// Example:
	//   bus.SubscribeWithPriority("edge-expert", ch, PriorityCritical)
	SubscribeWithPriority(id string, ch chan<- Frame, priority SubscriberPriority) error

	// Unsubscribe removes a subscriber by id.
	// Returns error if id not found or if bus is closed.
	Unsubscribe(id string) error

	// Publish sends frame to all subscribers (non-blocking).
	// Drops frame for subscribers whose channels are full.
	// Panics if bus is closed.
	//
	// Subscribers are processed in priority order (Critical first).
	// Under load, lower-priority subscribers drop frames before higher-priority ones.
	//
	// Note: frame.Ctx is ignored by Publish. Use PublishWithContext
	// if you need to set a context for the frame.
	Publish(frame Frame)

	// PublishWithContext sends frame with context to all subscribers (non-blocking).
	// The context is stored in frame.Ctx for downstream tracing/cancellation.
	// Drops frame for subscribers whose channels are full.
	// Panics if bus is closed.
	//
	// Subscribers are processed in priority order (Critical first).
	// Under load, lower-priority subscribers drop frames before higher-priority ones.
	//
	// Use cases:
	//   - Distributed tracing: Propagate trace ID through the pipeline
	//   - Cancellation: Signal downstream workers to abort processing
	//   - Deadlines: Expire stale frames based on context deadline
	//
	// Example:
	//   ctx, span := tracer.Start(context.Background(), "frame-publish")
	//   bus.PublishWithContext(ctx, frame)
	//   defer span.End()
	PublishWithContext(ctx context.Context, frame Frame)

	// Stats returns current bus statistics snapshot.
	Stats() BusStats

	// GetHealth returns the health state of a subscriber based on drop rate.
	//
	// Health states:
	//   - HealthHealthy:   Drop rate < 50% (normal operation)
	//   - HealthDegraded:  Drop rate 50-90% (falling behind)
	//   - HealthSaturated: Drop rate > 90% (critical overload)
	//   - HealthUnknown:   No activity yet (sent + dropped == 0)
	//
	// Returns HealthUnknown if subscriber ID not found.
	//
	// Example:
	//   health := bus.GetHealth("worker-1")
	//   if health == HealthSaturated {
	//       log.Error("worker-1 is saturated, restarting...")
	//       restartWorker("worker-1")
	//   }
	GetHealth(id string) SubscriberHealth

	// GetUnhealthySubscribers returns IDs of all degraded or saturated subscribers.
	//
	// Useful for proactive monitoring and alerting.
	// Returns empty slice if all subscribers are healthy.
	//
	// Example:
	//   unhealthy := bus.GetUnhealthySubscribers()
	//   for _, id := range unhealthy {
	//       log.Warn("unhealthy subscriber", "id", id, "health", bus.GetHealth(id))
	//   }
	GetUnhealthySubscribers() []string

	// Close stops the bus and prevents further operations.
	// Subsequent Subscribe/Unsubscribe will return ErrBusClosed.
	// Subsequent Publish will panic.
	Close() error
}

// subscriberEntry holds channel and priority for a subscriber.
type subscriberEntry struct {
	ch       chan<- Frame
	priority SubscriberPriority
}

// subscriberStats holds internal atomic counters.
type subscriberStats struct {
	sent    atomic.Uint64
	dropped atomic.Uint64
}

// bus is the concrete implementation of Bus.
type bus struct {
	mu          sync.RWMutex
	subscribers map[string]*subscriberEntry // Changed: now holds entry with priority
	stats       map[string]*subscriberStats
	closed      bool

	// Global counter (atomic - no lock needed in Publish)
	totalPublished atomic.Uint64
}

// New creates a new FrameBus.
func New() Bus {
	return &bus{
		subscribers: make(map[string]*subscriberEntry),
		stats:       make(map[string]*subscriberStats),
		closed:      false,
	}
}

// Subscribe registers a channel with default (Normal) priority.
func (b *bus) Subscribe(id string, ch chan<- Frame) error {
	return b.SubscribeWithPriority(id, ch, PriorityNormal)
}

// SubscribeWithPriority registers a channel with a specific priority level.
func (b *bus) SubscribeWithPriority(id string, ch chan<- Frame, priority SubscriberPriority) error {
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

	b.subscribers[id] = &subscriberEntry{
		ch:       ch,
		priority: priority,
	}
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
// Subscribers are processed in priority order (Critical first, BestEffort last).
// Under load, lower-priority subscribers drop frames before higher-priority ones.
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

	// Sort subscribers by priority (Critical first)
	sorted := b.sortSubscribersByPriority()

	// Distribute to subscribers in priority order
	for _, sub := range sorted {
		select {
		case sub.entry.ch <- frame:
			// Frame sent successfully
			b.stats[sub.id].sent.Add(1)
		default:
			// Channel full - drop frame
			b.stats[sub.id].dropped.Add(1)
		}
	}
}

// PublishWithContext sends frame with context to all subscribers (non-blocking).
//
// The context is stored in frame.Ctx before distribution. Subscribers can
// use the context for:
//   - Distributed tracing (extract trace ID from context)
//   - Request cancellation (check ctx.Done())
//   - Deadline enforcement (check ctx.Deadline())
//
// Subscribers are processed in priority order (Critical first, BestEffort last).
// Under load, lower-priority subscribers drop frames before higher-priority ones.
//
// For each subscriber:
//   - If channel has space: frame is sent, Sent counter incremented
//   - If channel is full: frame is dropped, Dropped counter incremented
//
// This method never blocks, even if all subscribers are slow.
// Panics if bus is closed (check with defer/recover if needed).
func (b *bus) PublishWithContext(ctx context.Context, frame Frame) {
	// Attach context to frame
	frame.Ctx = ctx

	b.totalPublished.Add(1)

	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		panic("publish on closed bus")
	}

	// Sort subscribers by priority (Critical first)
	sorted := b.sortSubscribersByPriority()

	// Distribute to subscribers in priority order
	for _, sub := range sorted {
		select {
		case sub.entry.ch <- frame:
			// Frame sent successfully
			b.stats[sub.id].sent.Add(1)
		default:
			// Channel full - drop frame
			b.stats[sub.id].dropped.Add(1)
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

		// Get priority from subscriber entry
		priority := PriorityNormal // Default if entry not found
		if entry, exists := b.subscribers[id]; exists {
			priority = entry.priority
		}

		result.Subscribers[id] = SubscriberStats{
			Sent:     sent,
			Dropped:  dropped,
			Priority: priority,
		}
	}

	result.TotalSent = totalSent
	result.TotalDropped = totalDropped

	return result
}

// GetHealth returns the health state of a subscriber based on drop rate.
//
// Health is computed from current stats:
//   - HealthHealthy:   Drop rate < 50%
//   - HealthDegraded:  Drop rate 50-90%
//   - HealthSaturated: Drop rate > 90%
//   - HealthUnknown:   No activity (sent + dropped == 0) or ID not found
//
// Thread-safe, can be called concurrently with Publish.
func (b *bus) GetHealth(id string) SubscriberHealth {
	b.mu.RLock()
	defer b.mu.RUnlock()

	stats, exists := b.stats[id]
	if !exists {
		return HealthUnknown
	}

	sent := stats.sent.Load()
	dropped := stats.dropped.Load()
	total := sent + dropped

	if total == 0 {
		return HealthUnknown
	}

	dropRate := float64(dropped) / float64(total)

	switch {
	case dropRate < 0.5:
		return HealthHealthy
	case dropRate < 0.9:
		return HealthDegraded
	default: // dropRate >= 0.9
		return HealthSaturated
	}
}

// GetUnhealthySubscribers returns IDs of all degraded or saturated subscribers.
//
// Returns:
//   - Empty slice if all subscribers are healthy or unknown
//   - IDs of subscribers with drop rate >= 50%
//
// Useful for proactive monitoring loops.
// Thread-safe, can be called concurrently with Publish.
func (b *bus) GetUnhealthySubscribers() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var unhealthy []string

	for id, stats := range b.stats {
		sent := stats.sent.Load()
		dropped := stats.dropped.Load()
		total := sent + dropped

		if total == 0 {
			continue // Skip unknown state
		}

		dropRate := float64(dropped) / float64(total)

		if dropRate >= 0.5 {
			unhealthy = append(unhealthy, id)
		}
	}

	return unhealthy
}

// sortedSubscriber holds a subscriber entry with its ID for sorting.
type sortedSubscriber struct {
	id    string
	entry *subscriberEntry
}

// sortSubscribersByPriority returns subscribers sorted by priority.
// Lower priority value = higher priority (Critical=0 first, BestEffort=3 last).
//
// Called within RLock, returns a slice to avoid holding lock during iteration.
func (b *bus) sortSubscribersByPriority() []sortedSubscriber {
	sorted := make([]sortedSubscriber, 0, len(b.subscribers))

	for id, entry := range b.subscribers {
		sorted = append(sorted, sortedSubscriber{
			id:    id,
			entry: entry,
		})
	}

	// Sort by priority (Critical first)
	// Using simple insertion sort for small N (typically 5-10 subscribers)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].entry.priority < sorted[j-1].entry.priority; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

	return sorted
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
