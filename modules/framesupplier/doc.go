// Package framesupplier implements just-in-time frame distribution
// with symmetric mailbox architecture for real-time video processing.
//
// # Philosophy
//
// "Drop frames, never queue. Latency > Completeness."
//
// Orion is a real-time AI inference system where staleness is worse than
// incompleteness. FrameSupplier extends this philosophy through symmetric
// JIT architecture: every level (publisher→supplier, supplier→workers)
// maintains "latest frame only", dropping stale frames to maintain
// real-time responsiveness.
//
// # Design Principles
//
//  1. Non-blocking Publish: Publish() never blocks, returns in ~1µs
//  2. Blocking Consume: Workers block until frame available (efficient, no busy-wait)
//  3. Zero-copy: Shared pointers, immutability contract (compete with GStreamer/DeepStream)
//  4. Batching: Threshold-based parallelism (sequential ≤8 workers, concurrent >8)
//  5. Operational Stats: Idle detection, drop counters (not benchmarking)
//
// # Architecture
//
// FrameSupplier sits between stream-capture and workers:
//
//	stream-capture → FrameSupplier → Workers (PersonDetector, PoseDetector, ...)
//	     (30fps)      Inbox Mailbox      Worker Slots (N)
//	                  Overwrite Policy    Overwrite Policy
//
// Each level uses mailbox pattern (sync.Cond, single-slot buffer, overwrite).
//
// See: docs/ARCHITECTURE.md (deep dive), docs/C4_MODEL.md (visual views)
//
// # Basic Usage
//
// Publisher side (stream-capture):
//
//	supplier := framesupplier.New()
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//
//	go func() {
//	    if err := supplier.Start(ctx); err != nil {
//	        log.Fatalf("Supplier failed: %v", err)
//	    }
//	}()
//
//	// Publish frames from GStreamer @ 30fps
//	for {
//	    frame := captureFromGStreamer()
//	    supplier.Publish(frame)  // Non-blocking, ~1µs
//	}
//
// Consumer side (worker):
//
//	readFunc := supplier.Subscribe("PersonDetector")
//	defer supplier.Unsubscribe("PersonDetector")
//
//	for {
//	    frame := readFunc()  // Blocks until frame available
//	    if frame == nil {
//	        break  // Graceful shutdown
//	    }
//	    result := runInference(frame)
//	    publishResult(result)
//	}
//
// # Monitoring
//
// Check operational health with Stats():
//
//	stats := supplier.Stats()
//
//	// Inbox drops (should be ~0)
//	if stats.InboxDrops > 0 {
//	    log.Warn("Distribution loop slow", "drops", stats.InboxDrops)
//	}
//
//	// Worker health
//	for workerID, workerStats := range stats.Workers {
//	    if workerStats.IsIdle {
//	        log.Warn("Worker idle", "id", workerID)
//	    }
//	    dropRate := float64(workerStats.TotalDrops) / float64(workerStats.LastConsumedSeq)
//	    log.Info("Worker drops", "id", workerID, "rate", dropRate)
//	}
//
// # Drop Semantics
//
// Drops are EXPECTED and HEALTHY in JIT architecture:
//
//   - Inbox drops: Should be ~0 (distribution is 330× faster than 30fps source)
//   - Worker drops: Expected when worker FPS < source FPS
//     Example: 30fps source → 1fps worker = 29 drops/sec (96.7% drop rate)
//     This is CORRECT behavior (worker gets latest frame, not 1-second-old frame)
//
// Drops are NOT errors. They indicate JIT semantics working correctly.
//
// # Zero-Copy Contract
//
// Frame.Data is shared by reference (not copied). IMMUTABILITY CONTRACT:
//
//   - Publisher: MUST NOT modify frame.Data after Publish()
//   - Workers: MUST NOT modify frame.Data (read-only)
//
// Enforcement: Documentation-based (runtime checks would add overhead).
//
// # Thread Safety
//
// All methods are thread-safe:
//
//   - Publish(): Safe for concurrent calls (typically 1 publisher)
//   - Subscribe(): Safe for concurrent calls (typically N workers)
//   - Unsubscribe(): Safe for concurrent calls
//   - Stats(): Safe for concurrent calls (returns snapshot)
//
// Worker's readFunc: Safe to call from single worker goroutine only.
//
// # Performance Characteristics
//
//   - Publish() latency: ~1µs (lock + pointer assign + signal)
//   - Distribution latency: ~100µs @ 64 workers (batching with threshold=8)
//   - Memory overhead: ~73 bytes per worker + 16KB transient (>8 workers)
//   - Zero allocations in steady state (pre-allocated slots)
//
// # Lifecycle
//
//  1. New(): Create supplier
//  2. Start(ctx): Begin distribution loop (blocks until ctx.Done or Stop)
//  3. Publish()/Subscribe(): Normal operation
//  4. Stop(): Graceful shutdown (blocks until distributionLoop exits)
//
// After Stop():
//   - Publish() becomes no-op (safe, but frames dropped)
//   - Subscribe() readFunc returns nil (workers detect shutdown)
//
// # Architectural Decisions
//
// This module is guided by:
//
//   - ADR-001: sync.Cond for Mailbox Semantics
//   - ADR-002: Zero-Copy Frame Sharing
//   - ADR-003: Batching with Threshold=8
//   - ADR-004: Symmetric JIT Architecture
//
// See: docs/ADR/ for rationale and alternatives considered.
//
// # Future Extensions (Out of Scope)
//
// Phase 2: Multi-stream support (streamID in Frame)
// Phase 3: Priority-based distribution (skip low-priority under load)
//
// These are NOT in current scope (YAGNI), but API designed for extensibility.
//
// # Module Context
//
// FrameSupplier is part of Orion 2.0 modular architecture:
//
//   - Bounded Context: Frame distribution only (not worker lifecycle, not inference)
//   - Dependencies: None (stdlib only)
//   - Clients: stream-capture (publisher), worker-lifecycle (consumers)
//
// See: OrionWork/CLAUDE.md (global context), modules/framesupplier/CLAUDE.md (module context)
package framesupplier
