// Package framebus provides non-blocking frame distribution to multiple subscribers.
//
// # Overview
//
// FrameBus implements a pub/sub pattern for video frames where a publisher distributes
// frames to multiple subscribers using Go channels. The key design principle is:
//
//	"Drop frames, never queue. Latency > Completeness."
//
// When a subscriber's channel is full, frames are intentionally dropped rather than queued,
// maintaining bounded latency for real-time video processing.
//
// # Basic Usage
//
// Create a bus and subscribe channels:
//
//	bus := framebus.New()
//	defer bus.Close()
//
//	workerCh := make(chan framebus.Frame, 5)
//	bus.Subscribe("worker-1", workerCh)
//
//	// Publish frames
//	for frame := range source {
//	    bus.Publish(frame)
//	}
//
// # Non-Blocking Semantics
//
// Publish() never blocks, even if all subscribers are slow:
//
//	bus.Publish(frame)  // Returns immediately
//
// If a subscriber's channel is full, the frame is dropped and tracked in stats.
//
// # Observability
//
// Stats provide global and per-subscriber metrics:
//
//	stats := bus.Stats()
//	fmt.Printf("Published: %d, Sent: %d, Dropped: %d\n",
//	    stats.TotalPublished, stats.TotalSent, stats.TotalDropped)
//
//	for id, sub := range stats.Subscribers {
//	    dropRate := float64(sub.Dropped) / float64(sub.Sent + sub.Dropped)
//	    fmt.Printf("%s: %.1f%% drop rate\n", id, dropRate*100)
//	}
//
// # Thread Safety
//
// All operations are thread-safe:
//   - Multiple goroutines can call Publish() concurrently
//   - Subscribe/Unsubscribe can be called while publishing
//   - Stats() can be called from any goroutine
//
// # Performance
//
// Publish() completes in microseconds (typically < 1µs per subscriber).
// Memory usage is constant, bounded by subscriber channel buffers.
//
// Benchmarks (single frame, 10 subscribers):
//   - Publish: ~1-2µs
//   - Stats: ~1-5µs
//
// # Design Decisions
//
// See docs/adr/ for detailed architectural decisions:
//   - ADR-001: Channel-based Subscriber Pattern
//   - ADR-002: Non-blocking Publish with Drop Policy
//   - ADR-003: Stats Tracking Design
//
// # Example
//
// See examples/basic/ for a complete working example simulating video processing
// with workers of different processing speeds.
package framebus
