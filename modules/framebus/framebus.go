// Package framebus provides non-blocking frame distribution for real-time video processing.
//
// Core Philosophy: "Drop frames, never queue. Latency > Completeness."
//
// FrameBus distributes video frames to multiple subscribers with configurable drop policies:
//   - DropNew: Backpressure-based dropping (channel buffer full → drop incoming frame)
//   - DropOld: Always latest (replace stored frame, never drop)
//
// Usage:
//
//	bus := framebus.New()
//	defer bus.Close()
//
//	// DropNew subscriber (current behavior)
//	ch := make(chan framebus.Frame, 5)
//	bus.Subscribe("worker-1", ch)
//
//	// DropOld subscriber (always latest)
//	receiver, _ := bus.SubscribeDropOld("worker-2")
//	defer receiver.Close()
//
//	// Publish frames
//	bus.Publish(framebus.Frame{Sequence: 1, Data: []byte("...")})
//
// Public API Stability:
//
// This package follows semantic versioning. The public API (types, interfaces, errors)
// is considered stable and will not change in backwards-incompatible ways without a
// major version bump. Internal implementation can evolve freely.
package framebus

import "github.com/e7canasta/orion-care-sensor/modules/framebus/internal/bus"

// New creates a new FrameBus instance with default configuration (Mutex-based)
// This is the only public constructor and part of the stable API
func New() Bus {
	return bus.New()
}
