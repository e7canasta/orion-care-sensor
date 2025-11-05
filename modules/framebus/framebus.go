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
// # Context Propagation (Distributed Tracing)
//
//	// Publish with context for tracing
//	ctx, span := tracer.Start(context.Background(), "frame-publish")
//	bus.PublishWithContext(ctx, frame)
//	defer span.End()
//
//	// Subscriber reads context
//	for frame := range workerCh {
//	    if frame.Ctx != nil {
//	        _, span := tracer.Start(frame.Ctx, "worker-process")
//	        defer span.End()
//	    }
//	    processFrame(frame)
//	}
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
	"github.com/visiona/orion/modules/framebus/internal/bus"
)

// Bus distributes frames to multiple subscribers with drop policy.
type Bus = bus.Bus

// Frame represents a video frame to be distributed.
type Frame = bus.Frame

// BusStats contains global and per-subscriber metrics.
type BusStats = bus.BusStats

// SubscriberStats tracks metrics for a single subscriber.
type SubscriberStats = bus.SubscriberStats

// SubscriberHealth represents the health state of a subscriber.
type SubscriberHealth = bus.SubscriberHealth

const (
	// HealthHealthy indicates normal operation with low drop rate (< 50%).
	HealthHealthy = bus.HealthHealthy

	// HealthDegraded indicates elevated drop rate (50-90%).
	HealthDegraded = bus.HealthDegraded

	// HealthSaturated indicates critical drop rate (> 90%).
	HealthSaturated = bus.HealthSaturated

	// HealthUnknown is returned for subscribers with no activity yet.
	HealthUnknown = bus.HealthUnknown
)

var (
	// ErrSubscriberExists is returned when Subscribe is called with a duplicate id.
	ErrSubscriberExists = bus.ErrSubscriberExists

	// ErrSubscriberNotFound is returned when Unsubscribe is called with unknown id.
	ErrSubscriberNotFound = bus.ErrSubscriberNotFound

	// ErrBusClosed is returned when operations are attempted on a closed bus.
	ErrBusClosed = bus.ErrBusClosed
)

// New creates a new FrameBus.
func New() Bus {
	return bus.New()
}

