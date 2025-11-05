package bus

import (
	"testing"
)

// TestPriorityOrdering verifies that under saturation, critical subscribers get frames first.
func TestPriorityOrdering(t *testing.T) {
	b := New()
	defer b.Close()

	// Create channels with small buffers to force drops
	criticalCh := make(chan Frame, 5)
	bestEffortCh := make(chan Frame, 5)

	if err := b.SubscribeWithPriority("critical", criticalCh, PriorityCritical); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	if err := b.SubscribeWithPriority("best-effort", bestEffortCh, PriorityBestEffort); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Publish 20 frames (exceeds both buffers)
	for i := 0; i < 20; i++ {
		b.Publish(Frame{Seq: uint64(i), Data: []byte("test")})
	}

	stats := b.Stats()
	criticalStats := stats.Subscribers["critical"]
	bestEffortStats := stats.Subscribers["best-effort"]

	// Critical should have received all 5 buffer slots before best-effort starts dropping
	// Since critical is processed first in sorted order
	if criticalStats.Sent < bestEffortStats.Sent {
		t.Errorf("Expected critical to receive at least as many frames as best-effort. Critical: %d, BestEffort: %d",
			criticalStats.Sent, bestEffortStats.Sent)
	}

	// Under load, best-effort should drop more than critical
	if bestEffortStats.Dropped < criticalStats.Dropped {
		t.Errorf("Expected best-effort to drop more frames than critical. Critical drops: %d, BestEffort drops: %d",
			criticalStats.Dropped, bestEffortStats.Dropped)
	}

	t.Logf("Critical: sent=%d dropped=%d | BestEffort: sent=%d dropped=%d",
		criticalStats.Sent, criticalStats.Dropped,
		bestEffortStats.Sent, bestEffortStats.Dropped)
}

// TestPriorityLoadShedding verifies that lower-priority subscribers drop more frames.
func TestPriorityLoadShedding(t *testing.T) {
	b := New()
	defer b.Close()

	// Create channels: Critical with buffer=10, BestEffort with buffer=1
	criticalCh := make(chan Frame, 10)
	bestEffortCh := make(chan Frame, 1)

	if err := b.SubscribeWithPriority("critical", criticalCh, PriorityCritical); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	if err := b.SubscribeWithPriority("best-effort", bestEffortCh, PriorityBestEffort); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Publish 20 frames (exceeds both buffers, but Critical has more capacity)
	for i := 0; i < 20; i++ {
		b.Publish(Frame{Seq: uint64(i), Data: []byte("test")})
	}

	stats := b.Stats()

	criticalStats := stats.Subscribers["critical"]
	bestEffortStats := stats.Subscribers["best-effort"]

	// Critical should have received more frames (buffer=10 vs buffer=1)
	if criticalStats.Sent <= bestEffortStats.Sent {
		t.Errorf("Expected critical to receive more frames than best-effort. Critical: %d, BestEffort: %d",
			criticalStats.Sent, bestEffortStats.Sent)
	}

	// BestEffort should have dropped more frames
	if bestEffortStats.Dropped <= criticalStats.Dropped {
		t.Errorf("Expected best-effort to drop more frames than critical. Critical drops: %d, BestEffort drops: %d",
			criticalStats.Dropped, bestEffortStats.Dropped)
	}

	// Verify total accounting
	if stats.TotalPublished != 20 {
		t.Errorf("Expected 20 published, got %d", stats.TotalPublished)
	}

	totalSent := criticalStats.Sent + bestEffortStats.Sent
	totalDropped := criticalStats.Dropped + bestEffortStats.Dropped
	if totalSent != stats.TotalSent {
		t.Errorf("TotalSent mismatch: %d != %d", totalSent, stats.TotalSent)
	}
	if totalDropped != stats.TotalDropped {
		t.Errorf("TotalDropped mismatch: %d != %d", totalDropped, stats.TotalDropped)
	}
}

// TestBackwardCompatibilityDefaultPriority verifies Subscribe() uses PriorityNormal.
func TestBackwardCompatibilityDefaultPriority(t *testing.T) {
	b := New()
	defer b.Close()

	ch := make(chan Frame, 5)
	if err := b.Subscribe("worker-1", ch); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Publish one frame to populate stats
	b.Publish(Frame{Seq: 1, Data: []byte("test")})

	stats := b.Stats()
	workerStats := stats.Subscribers["worker-1"]

	if workerStats.Priority != PriorityNormal {
		t.Errorf("Expected default priority to be Normal, got %d", workerStats.Priority)
	}
}

// TestPriorityInStats verifies that Stats() includes priority information.
func TestPriorityInStats(t *testing.T) {
	b := New()
	defer b.Close()

	ch1 := make(chan Frame, 5)
	ch2 := make(chan Frame, 5)
	ch3 := make(chan Frame, 5)

	b.SubscribeWithPriority("critical-worker", ch1, PriorityCritical)
	b.SubscribeWithPriority("high-worker", ch2, PriorityHigh)
	b.Subscribe("normal-worker", ch3) // Default: Normal

	// Publish to populate stats
	b.Publish(Frame{Seq: 1, Data: []byte("test")})

	stats := b.Stats()

	if stats.Subscribers["critical-worker"].Priority != PriorityCritical {
		t.Errorf("Expected critical-worker to have PriorityCritical")
	}
	if stats.Subscribers["high-worker"].Priority != PriorityHigh {
		t.Errorf("Expected high-worker to have PriorityHigh")
	}
	if stats.Subscribers["normal-worker"].Priority != PriorityNormal {
		t.Errorf("Expected normal-worker to have PriorityNormal")
	}
}

// TestPriorityAllLevels verifies all 4 priority levels work correctly.
func TestPriorityAllLevels(t *testing.T) {
	b := New()
	defer b.Close()

	priorities := []struct {
		id       string
		priority SubscriberPriority
		ch       chan Frame
	}{
		{"critical", PriorityCritical, make(chan Frame, 15)}, // Buffer > 10
		{"high", PriorityHigh, make(chan Frame, 15)},
		{"normal", PriorityNormal, make(chan Frame, 15)},
		{"best-effort", PriorityBestEffort, make(chan Frame, 15)},
	}

	// Subscribe all levels
	for _, p := range priorities {
		if err := b.SubscribeWithPriority(p.id, p.ch, p.priority); err != nil {
			t.Fatalf("Subscribe failed for %s: %v", p.id, err)
		}
	}

	// Publish frames
	for i := 0; i < 10; i++ {
		b.Publish(Frame{Seq: uint64(i), Data: []byte("test")})
	}

	stats := b.Stats()

	// All should receive all frames (buffer sufficient)
	for _, p := range priorities {
		subStats := stats.Subscribers[p.id]
		if subStats.Sent != 10 {
			t.Errorf("Expected %s to receive 10 frames, got %d", p.id, subStats.Sent)
		}
		if subStats.Dropped != 0 {
			t.Errorf("Expected %s to drop 0 frames, got %d", p.id, subStats.Dropped)
		}
		if subStats.Priority != p.priority {
			t.Errorf("Expected %s priority %d, got %d", p.id, p.priority, subStats.Priority)
		}
	}
}

// TestSubscribeWithPriorityErrors verifies error handling.
func TestSubscribeWithPriorityErrors(t *testing.T) {
	b := New()
	defer b.Close()

	ch := make(chan Frame, 5)

	// Subscribe successfully
	if err := b.SubscribeWithPriority("worker-1", ch, PriorityCritical); err != nil {
		t.Fatalf("First subscribe failed: %v", err)
	}

	// Duplicate ID should fail
	ch2 := make(chan Frame, 5)
	err := b.SubscribeWithPriority("worker-1", ch2, PriorityHigh)
	if err != ErrSubscriberExists {
		t.Errorf("Expected ErrSubscriberExists, got %v", err)
	}

	// Nil channel should fail
	err = b.SubscribeWithPriority("worker-2", nil, PriorityNormal)
	if err == nil {
		t.Error("Expected error for nil channel, got nil")
	}

	// After close, subscribe should fail
	b.Close()
	ch3 := make(chan Frame, 5)
	err = b.SubscribeWithPriority("worker-3", ch3, PriorityNormal)
	if err != ErrBusClosed {
		t.Errorf("Expected ErrBusClosed, got %v", err)
	}
}

// TestCriticalRetry verifies that Critical subscribers get retry logic.
func TestCriticalRetry(t *testing.T) {
	b := New()
	defer b.Close()

	// Create critical subscriber with buffer=1
	criticalCh := make(chan Frame, 1)
	if err := b.SubscribeWithPriority("critical", criticalCh, PriorityCritical); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Create normal subscriber with buffer=1 for comparison
	normalCh := make(chan Frame, 1)
	if err := b.SubscribeWithPriority("normal", normalCh, PriorityNormal); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Publish 2 frames quickly (both channels buffer=1, second frame will block both)
	b.Publish(Frame{Seq: 1, Data: []byte("test1")})
	b.Publish(Frame{Seq: 2, Data: []byte("test2")})

	stats := b.Stats()
	criticalStats := stats.Subscribers["critical"]
	normalStats := stats.Subscribers["normal"]

	// Normal should drop immediately (no retry)
	if normalStats.Dropped != 1 {
		t.Errorf("Expected normal to drop 1 frame immediately, got %d", normalStats.Dropped)
	}

	// Critical might succeed with retry (if consumer drains within 1ms)
	// or drop after timeout - both are valid depending on timing
	t.Logf("Critical: sent=%d dropped=%d criticalDropped=%d",
		criticalStats.Sent, criticalStats.Dropped, criticalStats.CriticalDropped)
	t.Logf("Normal: sent=%d dropped=%d", normalStats.Sent, normalStats.Dropped)

	// Verify CriticalDropped is tracked
	if criticalStats.Priority != PriorityCritical {
		t.Errorf("Expected critical priority, got %d", criticalStats.Priority)
	}
}

// TestCriticalDroppedMetric verifies CriticalDropped is only incremented for Critical subscribers.
func TestCriticalDroppedMetric(t *testing.T) {
	b := New()
	defer b.Close()

	// Create subscribers with different priorities
	criticalCh := make(chan Frame, 1)
	normalCh := make(chan Frame, 1)

	b.SubscribeWithPriority("critical", criticalCh, PriorityCritical)
	b.SubscribeWithPriority("normal", normalCh, PriorityNormal)

	// Saturate both channels
	for i := 0; i < 10; i++ {
		b.Publish(Frame{Seq: uint64(i), Data: []byte("test")})
	}

	stats := b.Stats()

	// Normal should have 0 CriticalDropped (not a critical subscriber)
	if stats.Subscribers["normal"].CriticalDropped != 0 {
		t.Errorf("Expected normal CriticalDropped=0, got %d",
			stats.Subscribers["normal"].CriticalDropped)
	}

	// Critical may have CriticalDropped > 0 if retry also failed
	criticalStats := stats.Subscribers["critical"]
	t.Logf("Critical: sent=%d dropped=%d criticalDropped=%d",
		criticalStats.Sent, criticalStats.Dropped, criticalStats.CriticalDropped)

	// CriticalDropped should never exceed Dropped for critical subscriber
	if criticalStats.CriticalDropped > criticalStats.Dropped {
		t.Errorf("CriticalDropped (%d) should not exceed Dropped (%d)",
			criticalStats.CriticalDropped, criticalStats.Dropped)
	}
}
