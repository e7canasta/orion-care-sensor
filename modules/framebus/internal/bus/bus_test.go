package bus

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestBasicPublishSubscribe verifies basic functionality.
func TestBasicPublishSubscribe(t *testing.T) {
	bus := New()
	defer bus.Close()

	ch := make(chan Frame, 10)
	if err := bus.Subscribe("test", ch); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	frame := Frame{Seq: 1, Data: []byte("test")}
	bus.Publish(frame)

	select {
	case received := <-ch:
		if received.Seq != frame.Seq {
			t.Errorf("Expected seq %d, got %d", frame.Seq, received.Seq)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for frame")
	}
}

// TestNonBlockingPublish verifies Publish never blocks.
func TestNonBlockingPublish(t *testing.T) {
	bus := New()
	defer bus.Close()

	// Subscribe with buffer=1
	ch := make(chan Frame, 1)
	bus.Subscribe("slow", ch)

	// Publish 2 frames - second should drop, not block
	frame1 := Frame{Seq: 1}
	frame2 := Frame{Seq: 2}

	done := make(chan bool)
	go func() {
		bus.Publish(frame1) // Should succeed
		bus.Publish(frame2) // Should drop (buffer full)
		done <- true
	}()

	select {
	case <-done:
		// Success - Publish completed without blocking
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Publish blocked (should be non-blocking)")
	}

	// Verify first frame was sent
	received := <-ch
	if received.Seq != 1 {
		t.Errorf("Expected seq 1, got %d", received.Seq)
	}

	// Verify stats show 1 sent, 1 dropped
	stats := bus.Stats()
	subStats := stats.Subscribers["slow"]
	if subStats.Sent != 1 {
		t.Errorf("Expected 1 sent, got %d", subStats.Sent)
	}
	if subStats.Dropped != 1 {
		t.Errorf("Expected 1 dropped, got %d", subStats.Dropped)
	}
}

// TestStatsAccuracy verifies stats match actual behavior.
func TestStatsAccuracy(t *testing.T) {
	bus := New()
	defer bus.Close()

	// Create 3 subscribers with different buffer sizes
	ch1 := make(chan Frame, 10) // Large buffer
	ch2 := make(chan Frame, 1)  // Small buffer (will drop)
	ch3 := make(chan Frame, 10) // Large buffer

	bus.Subscribe("worker-1", ch1)
	bus.Subscribe("worker-2", ch2)
	bus.Subscribe("worker-3", ch3)

	// Publish 5 frames
	for i := uint64(1); i <= 5; i++ {
		bus.Publish(Frame{Seq: i})
	}

	stats := bus.Stats()

	// Verify global stats
	if stats.TotalPublished != 5 {
		t.Errorf("Expected 5 published, got %d", stats.TotalPublished)
	}

	// Verify conservation law: TotalSent + TotalDropped == TotalPublished * Subscribers
	expected := stats.TotalPublished * uint64(len(stats.Subscribers))
	actual := stats.TotalSent + stats.TotalDropped
	if actual != expected {
		t.Errorf("Conservation law violated: %d sent + %d dropped != %d published × %d subscribers",
			stats.TotalSent, stats.TotalDropped, stats.TotalPublished, len(stats.Subscribers))
	}

	// Worker 1 and 3 should have received all frames (buffer=10)
	if stats.Subscribers["worker-1"].Sent != 5 {
		t.Errorf("Worker-1 expected 5 sent, got %d", stats.Subscribers["worker-1"].Sent)
	}
	if stats.Subscribers["worker-3"].Sent != 5 {
		t.Errorf("Worker-3 expected 5 sent, got %d", stats.Subscribers["worker-3"].Sent)
	}

	// Worker 2 should have dropped some (buffer=1)
	w2 := stats.Subscribers["worker-2"]
	if w2.Dropped < 3 {
		t.Errorf("Worker-2 expected at least 3 drops, got %d", w2.Dropped)
	}
}

// TestSubscribeDuplicateID verifies error handling.
func TestSubscribeDuplicateID(t *testing.T) {
	bus := New()
	defer bus.Close()

	ch1 := make(chan Frame, 1)
	ch2 := make(chan Frame, 1)

	if err := bus.Subscribe("test", ch1); err != nil {
		t.Fatalf("First subscribe failed: %v", err)
	}

	err := bus.Subscribe("test", ch2)
	if err != ErrSubscriberExists {
		t.Errorf("Expected ErrSubscriberExists, got %v", err)
	}
}

// TestUnsubscribe verifies unsubscribe functionality.
func TestUnsubscribe(t *testing.T) {
	bus := New()
	defer bus.Close()

	ch := make(chan Frame, 1)
	bus.Subscribe("test", ch)

	// Verify subscriber exists
	stats := bus.Stats()
	if len(stats.Subscribers) != 1 {
		t.Fatalf("Expected 1 subscriber, got %d", len(stats.Subscribers))
	}

	// Unsubscribe
	if err := bus.Unsubscribe("test"); err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}

	// Verify subscriber removed
	stats = bus.Stats()
	if len(stats.Subscribers) != 0 {
		t.Errorf("Expected 0 subscribers, got %d", len(stats.Subscribers))
	}

	// Publish should not send to unsubscribed channel
	bus.Publish(Frame{Seq: 1})

	select {
	case <-ch:
		t.Error("Received frame after unsubscribe")
	case <-time.After(50 * time.Millisecond):
		// Expected - no frame received
	}
}

// TestUnsubscribeNotFound verifies error handling.
func TestUnsubscribeNotFound(t *testing.T) {
	bus := New()
	defer bus.Close()

	err := bus.Unsubscribe("nonexistent")
	if err != ErrSubscriberNotFound {
		t.Errorf("Expected ErrSubscriberNotFound, got %v", err)
	}
}

// TestMultipleSubscribers verifies fan-out to multiple channels.
func TestMultipleSubscribers(t *testing.T) {
	bus := New()
	defer bus.Close()

	// Create 10 subscribers
	channels := make([]chan Frame, 10)
	for i := 0; i < 10; i++ {
		ch := make(chan Frame, 5)
		channels[i] = ch
		bus.Subscribe(string(rune('A'+i)), ch)
	}

	// Publish one frame
	frame := Frame{Seq: 42}
	bus.Publish(frame)

	// Verify all subscribers received the frame
	for i, ch := range channels {
		select {
		case received := <-ch:
			if received.Seq != 42 {
				t.Errorf("Subscriber %d: expected seq 42, got %d", i, received.Seq)
			}
		case <-time.After(1 * time.Second):
			t.Errorf("Subscriber %d: timeout waiting for frame", i)
		}
	}

	// Verify stats
	stats := bus.Stats()
	if stats.TotalPublished != 1 {
		t.Errorf("Expected 1 published, got %d", stats.TotalPublished)
	}
	if stats.TotalSent != 10 {
		t.Errorf("Expected 10 sent (1 frame × 10 subscribers), got %d", stats.TotalSent)
	}
	if stats.TotalDropped != 0 {
		t.Errorf("Expected 0 dropped, got %d", stats.TotalDropped)
	}
}

// TestConcurrentPublish verifies thread safety with multiple publishers.
func TestConcurrentPublish(t *testing.T) {
	bus := New()
	defer bus.Close()

	ch := make(chan Frame, 1000)
	bus.Subscribe("test", ch)

	// Spawn 10 goroutines publishing 100 frames each
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				frame := Frame{Seq: uint64(id*100 + j)}
				bus.Publish(frame)
			}
		}(i)
	}

	wg.Wait()

	// Verify stats
	stats := bus.Stats()
	if stats.TotalPublished != 1000 {
		t.Errorf("Expected 1000 published, got %d", stats.TotalPublished)
	}

	// Verify conservation law
	subStats := stats.Subscribers["test"]
	if subStats.Sent+subStats.Dropped != 1000 {
		t.Errorf("Expected 1000 total (sent+dropped), got %d", subStats.Sent+subStats.Dropped)
	}
}

// TestConcurrentSubscribe verifies thread safety with dynamic subscribers.
func TestConcurrentSubscribe(t *testing.T) {
	bus := New()
	defer bus.Close()

	var wg sync.WaitGroup

	// Goroutine 1: Continuously publish
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			bus.Publish(Frame{Seq: uint64(i)})
			time.Sleep(1 * time.Millisecond)
		}
	}()

	// Goroutine 2: Add/remove subscribers dynamically
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			ch := make(chan Frame, 10)
			id := string(rune('A' + i))
			bus.Subscribe(id, ch)
			time.Sleep(5 * time.Millisecond)
			bus.Unsubscribe(id)
		}
	}()

	wg.Wait()

	// If we got here without panic/deadlock, thread safety works
	stats := bus.Stats()
	if stats.TotalPublished != 100 {
		t.Errorf("Expected 100 published, got %d", stats.TotalPublished)
	}
}

// TestClosedBus verifies behavior after Close().
func TestClosedBus(t *testing.T) {
	bus := New()
	ch := make(chan Frame, 1)
	bus.Subscribe("test", ch)

	// Close the bus
	if err := bus.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Subscribe should fail
	err := bus.Subscribe("new", make(chan Frame, 1))
	if err != ErrBusClosed {
		t.Errorf("Expected ErrBusClosed, got %v", err)
	}

	// Unsubscribe should fail
	err = bus.Unsubscribe("test")
	if err != ErrBusClosed {
		t.Errorf("Expected ErrBusClosed, got %v", err)
	}

	// Stats should still work
	stats := bus.Stats()
	if stats.TotalPublished != 0 {
		t.Errorf("Expected 0 published, got %d", stats.TotalPublished)
	}

	// Publish should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic on Publish after Close")
		}
	}()
	bus.Publish(Frame{Seq: 1})
}

// TestStatsMonotonicity verifies counters only increase.
func TestStatsMonotonicity(t *testing.T) {
	bus := New()
	defer bus.Close()

	ch := make(chan Frame, 1)
	bus.Subscribe("test", ch)

	prevStats := bus.Stats()

	for i := 0; i < 10; i++ {
		bus.Publish(Frame{Seq: uint64(i)})

		stats := bus.Stats()

		// Verify monotonicity
		if stats.TotalPublished < prevStats.TotalPublished {
			t.Error("TotalPublished decreased (not monotonic)")
		}
		if stats.TotalSent < prevStats.TotalSent {
			t.Error("TotalSent decreased (not monotonic)")
		}
		if stats.TotalDropped < prevStats.TotalDropped {
			t.Error("TotalDropped decreased (not monotonic)")
		}

		prevStats = stats
	}
}

// TestNilChannelSubscribe verifies error handling.
func TestNilChannelSubscribe(t *testing.T) {
	bus := New()
	defer bus.Close()

	err := bus.Subscribe("test", nil)
	if err == nil {
		t.Error("Expected error when subscribing with nil channel")
	}
}

// TestIdempotentClose verifies Close can be called multiple times.
func TestIdempotentClose(t *testing.T) {
	bus := New()

	if err := bus.Close(); err != nil {
		t.Fatalf("First close failed: %v", err)
	}

	if err := bus.Close(); err != nil {
		t.Fatalf("Second close failed: %v", err)
	}
}

// BenchmarkPublishSingleSubscriber measures Publish performance.
func BenchmarkPublishSingleSubscriber(b *testing.B) {
	bus := New()
	defer bus.Close()

	ch := make(chan Frame, 1000)
	bus.Subscribe("bench", ch)

	frame := Frame{Seq: 1, Data: make([]byte, 100)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Publish(frame)
	}
}

// BenchmarkPublishMultipleSubscribers measures fan-out performance.
func BenchmarkPublishMultipleSubscribers(b *testing.B) {
	bus := New()
	defer bus.Close()

	// 10 subscribers
	for i := 0; i < 10; i++ {
		ch := make(chan Frame, 1000)
		bus.Subscribe(string(rune('A'+i)), ch)
	}

	frame := Frame{Seq: 1, Data: make([]byte, 100)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Publish(frame)
	}
}

// BenchmarkStats measures Stats() performance.
func BenchmarkStats(b *testing.B) {
	bus := New()
	defer bus.Close()

	// 100 subscribers
	for i := 0; i < 100; i++ {
		ch := make(chan Frame, 10)
		bus.Subscribe(string(rune(i)), ch)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bus.Stats()
	}
}

// TestPublishWithContext verifies context propagation.
func TestPublishWithContext(t *testing.T) {
	bus := New()
	defer bus.Close()

	ch := make(chan Frame, 10)
	if err := bus.Subscribe("test", ch); err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Create context with value
	type ctxKey string
	ctx := context.WithValue(context.Background(), ctxKey("trace_id"), "trace-123")

	frame := Frame{Seq: 1, Data: []byte("test")}
	bus.PublishWithContext(ctx, frame)

	select {
	case received := <-ch:
		if received.Ctx == nil {
			t.Fatal("Expected context to be set, got nil")
		}
		traceID := received.Ctx.Value(ctxKey("trace_id"))
		if traceID != "trace-123" {
			t.Errorf("Expected trace_id=trace-123, got %v", traceID)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for frame")
	}
}

// TestPublishWithContextNil verifies backward compatibility with nil context.
func TestPublishWithContextNil(t *testing.T) {
	bus := New()
	defer bus.Close()

	ch := make(chan Frame, 10)
	bus.Subscribe("test", ch)

	frame := Frame{Seq: 1, Data: []byte("test")}
	// Old code path: Publish doesn't set context
	bus.Publish(frame)

	select {
	case received := <-ch:
		if received.Ctx != nil {
			t.Errorf("Expected nil context from Publish(), got %v", received.Ctx)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for frame")
	}
}

// TestPublishWithContextCancellation verifies cancellation propagation.
func TestPublishWithContextCancellation(t *testing.T) {
	bus := New()
	defer bus.Close()

	ch := make(chan Frame, 10)
	bus.Subscribe("test", ch)

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	frame := Frame{Seq: 1, Data: []byte("test")}
	bus.PublishWithContext(ctx, frame)

	select {
	case received := <-ch:
		if received.Ctx == nil {
			t.Fatal("Expected context to be set")
		}
		// Verify context is cancelled
		select {
		case <-received.Ctx.Done():
			// Expected
			if received.Ctx.Err() != context.Canceled {
				t.Errorf("Expected Canceled error, got %v", received.Ctx.Err())
			}
		default:
			t.Error("Expected context to be cancelled")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for frame")
	}
}

// TestPublishWithContextDeadline verifies deadline propagation.
func TestPublishWithContextDeadline(t *testing.T) {
	bus := New()
	defer bus.Close()

	ch := make(chan Frame, 10)
	bus.Subscribe("test", ch)

	// Create context with deadline
	deadline := time.Now().Add(100 * time.Millisecond)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	frame := Frame{Seq: 1, Data: []byte("test")}
	bus.PublishWithContext(ctx, frame)

	select {
	case received := <-ch:
		if received.Ctx == nil {
			t.Fatal("Expected context to be set")
		}
		// Verify deadline is propagated
		receivedDeadline, ok := received.Ctx.Deadline()
		if !ok {
			t.Error("Expected deadline to be set")
		}
		if !receivedDeadline.Equal(deadline) {
			t.Errorf("Expected deadline %v, got %v", deadline, receivedDeadline)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for frame")
	}
}

// TestPublishWithContextNonBlocking verifies PublishWithContext doesn't block.
func TestPublishWithContextNonBlocking(t *testing.T) {
	bus := New()
	defer bus.Close()

	// Subscribe with buffer=1
	ch := make(chan Frame, 1)
	bus.Subscribe("slow", ch)

	ctx := context.Background()
	frame1 := Frame{Seq: 1}
	frame2 := Frame{Seq: 2}

	done := make(chan bool)
	go func() {
		bus.PublishWithContext(ctx, frame1) // Should succeed
		bus.PublishWithContext(ctx, frame2) // Should drop (buffer full)
		done <- true
	}()

	// Verify non-blocking (should complete quickly)
	select {
	case <-done:
		// Success - did not block
	case <-time.After(100 * time.Millisecond):
		t.Fatal("PublishWithContext blocked")
	}

	// Verify stats
	stats := bus.Stats()
	if stats.Subscribers["slow"].Sent != 1 {
		t.Errorf("Expected 1 sent, got %d", stats.Subscribers["slow"].Sent)
	}
	if stats.Subscribers["slow"].Dropped != 1 {
		t.Errorf("Expected 1 dropped, got %d", stats.Subscribers["slow"].Dropped)
	}
}

// BenchmarkPublishWithContext measures PublishWithContext overhead.
func BenchmarkPublishWithContext(b *testing.B) {
	bus := New()
	defer bus.Close()

	// 10 subscribers
	for i := 0; i < 10; i++ {
		ch := make(chan Frame, 100)
		bus.Subscribe(string(rune(i)), ch)
	}

	ctx := context.Background()
	frame := Frame{Seq: 1, Data: make([]byte, 1024)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.PublishWithContext(ctx, frame)
	}
}

// BenchmarkPublishWithContextVsPublish compares overhead.
func BenchmarkPublishWithContextVsPublish(b *testing.B) {
	bus := New()
	defer bus.Close()

	// 10 subscribers
	for i := 0; i < 10; i++ {
		ch := make(chan Frame, 100)
		bus.Subscribe(string(rune(i)), ch)
	}

	ctx := context.Background()
	frame := Frame{Seq: 1, Data: make([]byte, 1024)}

	b.Run("Publish", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			bus.Publish(frame)
		}
	})

	b.Run("PublishWithContext", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			bus.PublishWithContext(ctx, frame)
		}
	})
}

// TestGetHealthHealthy verifies HealthHealthy state (< 50% drops).
func TestGetHealthHealthy(t *testing.T) {
	bus := New()
	defer bus.Close()

	// Buffer size 10 - will receive all frames
	ch := make(chan Frame, 10)
	bus.Subscribe("healthy", ch)

	// Publish 10 frames (all should be sent)
	for i := 0; i < 10; i++ {
		bus.Publish(Frame{Seq: uint64(i)})
	}

	health := bus.GetHealth("healthy")
	if health != HealthHealthy {
		t.Errorf("Expected HealthHealthy, got %v", health)
	}

	stats := bus.Stats()
	if stats.Subscribers["healthy"].Sent != 10 {
		t.Errorf("Expected 10 sent, got %d", stats.Subscribers["healthy"].Sent)
	}
	if stats.Subscribers["healthy"].Dropped != 0 {
		t.Errorf("Expected 0 dropped, got %d", stats.Subscribers["healthy"].Dropped)
	}
}

// TestGetHealthDegraded verifies HealthDegraded state (50-90% drops).
func TestGetHealthDegraded(t *testing.T) {
	bus := New()
	defer bus.Close()

	// Buffer size 2 - will drop some frames
	ch := make(chan Frame, 2)
	bus.Subscribe("degraded", ch)

	// Publish 10 frames (some will drop, achieving 50-90% drop rate)
	for i := 0; i < 10; i++ {
		bus.Publish(Frame{Seq: uint64(i)})
	}

	health := bus.GetHealth("degraded")
	stats := bus.Stats()
	sub := stats.Subscribers["degraded"]

	dropRate := float64(sub.Dropped) / float64(sub.Sent+sub.Dropped)

	// Verify drop rate is in degraded range
	if dropRate < 0.5 || dropRate >= 0.9 {
		t.Errorf("Expected drop rate 0.5-0.9 for degraded state, got %.2f", dropRate)
	}

	if health != HealthDegraded {
		t.Errorf("Expected HealthDegraded with drop rate %.2f, got %v", dropRate, health)
	}
}

// TestGetHealthSaturated verifies HealthSaturated state (> 90% drops).
func TestGetHealthSaturated(t *testing.T) {
	bus := New()
	defer bus.Close()

	// Buffer size 1 - will drop most frames
	ch := make(chan Frame, 1)
	bus.Subscribe("saturated", ch)

	// Publish 100 frames (will drop > 90%)
	for i := 0; i < 100; i++ {
		bus.Publish(Frame{Seq: uint64(i)})
	}

	health := bus.GetHealth("saturated")
	stats := bus.Stats()
	sub := stats.Subscribers["saturated"]

	dropRate := float64(sub.Dropped) / float64(sub.Sent+sub.Dropped)

	// Verify drop rate is saturated (> 90%)
	if dropRate < 0.9 {
		t.Errorf("Expected drop rate > 0.9 for saturated state, got %.2f", dropRate)
	}

	if health != HealthSaturated {
		t.Errorf("Expected HealthSaturated with drop rate %.2f, got %v", dropRate, health)
	}
}

// TestGetHealthUnknown verifies HealthUnknown state.
func TestGetHealthUnknown(t *testing.T) {
	bus := New()
	defer bus.Close()

	ch := make(chan Frame, 10)
	bus.Subscribe("unknown", ch)

	// No frames published yet
	health := bus.GetHealth("unknown")
	if health != HealthUnknown {
		t.Errorf("Expected HealthUnknown (no activity), got %v", health)
	}

	// Non-existent subscriber
	health = bus.GetHealth("does-not-exist")
	if health != HealthUnknown {
		t.Errorf("Expected HealthUnknown (not found), got %v", health)
	}
}

// TestGetUnhealthySubscribers verifies unhealthy subscriber detection.
func TestGetUnhealthySubscribers(t *testing.T) {
	bus := New()
	defer bus.Close()

	// Healthy subscriber (buffer 100 - won't fill)
	ch1 := make(chan Frame, 100)
	bus.Subscribe("healthy-1", ch1)

	// Degraded subscriber (buffer 2 - will drop 50-90%)
	ch2 := make(chan Frame, 2)
	bus.Subscribe("degraded-1", ch2)

	// Saturated subscriber (buffer 1 - will drop > 90%)
	ch3 := make(chan Frame, 1)
	bus.Subscribe("saturated-1", ch3)

	// Publish 50 frames
	for i := 0; i < 50; i++ {
		bus.Publish(Frame{Seq: uint64(i)})
	}

	unhealthy := bus.GetUnhealthySubscribers()

	// Should include degraded-1 and saturated-1, not healthy-1
	if len(unhealthy) != 2 {
		t.Errorf("Expected 2 unhealthy subscribers, got %d: %v", len(unhealthy), unhealthy)
	}

	// Verify specific IDs
	found := make(map[string]bool)
	for _, id := range unhealthy {
		found[id] = true
	}

	if !found["degraded-1"] {
		t.Error("Expected degraded-1 in unhealthy list")
	}
	if !found["saturated-1"] {
		t.Error("Expected saturated-1 in unhealthy list")
	}
	if found["healthy-1"] {
		t.Error("Did not expect healthy-1 in unhealthy list")
	}
}

// TestGetUnhealthySubscribersEmpty verifies empty slice when all healthy.
func TestGetUnhealthySubscribersEmpty(t *testing.T) {
	bus := New()
	defer bus.Close()

	// All subscribers healthy (large buffers)
	for i := 0; i < 5; i++ {
		ch := make(chan Frame, 100)
		bus.Subscribe(string(rune(i)), ch)
	}

	// Publish 10 frames (all will be sent)
	for i := 0; i < 10; i++ {
		bus.Publish(Frame{Seq: uint64(i)})
	}

	unhealthy := bus.GetUnhealthySubscribers()
	if len(unhealthy) != 0 {
		t.Errorf("Expected empty unhealthy list, got %v", unhealthy)
	}
}

// BenchmarkCriticalRetrySuccess measures overhead when Critical retry succeeds.
func BenchmarkCriticalRetrySuccess(b *testing.B) {
	bus := New()
	defer bus.Close()

	// Critical subscriber with buffer=10 (large enough to avoid drops in benchmark)
	criticalCh := make(chan Frame, 10)
	bus.SubscribeWithPriority("critical", criticalCh, PriorityCritical)

	// Consumer goroutine draining channel
	go func() {
		for range criticalCh {
			// Drain
		}
	}()

	frame := Frame{Seq: 1, Data: make([]byte, 1024)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Publish(frame)
	}
}

// BenchmarkCriticalRetryTimeout measures overhead when Critical retry times out.
func BenchmarkCriticalRetryTimeout(b *testing.B) {
	bus := New()
	defer bus.Close()

	// Critical subscriber with buffer=1 (will saturate quickly)
	criticalCh := make(chan Frame, 1)
	bus.SubscribeWithPriority("critical", criticalCh, PriorityCritical)

	// No consumer - channel will fill and trigger retry timeout

	frame := Frame{Seq: 1, Data: make([]byte, 1024)}

	// Pre-fill channel
	criticalCh <- frame

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Publish(frame) // Will trigger retry + 1ms timeout
	}
}

// BenchmarkPriorityVsNoPriority compares overhead of priority sorting.
func BenchmarkPriorityVsNoPriority(b *testing.B) {
	b.Run("WithPriority", func(b *testing.B) {
		bus := New()
		defer bus.Close()

		// 10 subscribers with mixed priorities
		for i := 0; i < 10; i++ {
			ch := make(chan Frame, 100)
			priority := SubscriberPriority(i % 4) // Mix of 4 priority levels
			bus.SubscribeWithPriority(fmt.Sprintf("worker-%d", i), ch, priority)

			// Consumer draining
			go func(ch chan Frame) {
				for range ch {
				}
			}(ch)
		}

		frame := Frame{Seq: 1, Data: make([]byte, 1024)}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			bus.Publish(frame)
		}
	})

	b.Run("AllNormalPriority", func(b *testing.B) {
		bus := New()
		defer bus.Close()

		// 10 subscribers, all Normal priority (minimal sorting overhead)
		for i := 0; i < 10; i++ {
			ch := make(chan Frame, 100)
			bus.Subscribe(fmt.Sprintf("worker-%d", i), ch) // All Normal

			// Consumer draining
			go func(ch chan Frame) {
				for range ch {
				}
			}(ch)
		}

		frame := Frame{Seq: 1, Data: make([]byte, 1024)}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			bus.Publish(frame)
		}
	})
}

// TestLazyRebuild verifies that cache is rebuilt lazily on first Publish().
func TestLazyRebuild(t *testing.T) {
	b := New().(*bus)
	defer b.Close()

	// Multiple subscribes without Publish
	ch1 := make(chan Frame, 10)
	ch2 := make(chan Frame, 10)
	ch3 := make(chan Frame, 10)

	b.SubscribeWithPriority("critical", ch1, PriorityCritical)
	b.SubscribeWithPriority("normal", ch2, PriorityNormal)
	b.SubscribeWithPriority("best-effort", ch3, PriorityBestEffort)

	// Cache should be dirty after subscribes
	if !b.cacheDirty.Load() {
		t.Error("Expected cache to be dirty after subscribes")
	}

	// Cache should still be empty (not rebuilt yet)
	if len(b.sortedCache) != 0 {
		t.Errorf("Expected empty cache before Publish, got %d entries", len(b.sortedCache))
	}

	// First Publish triggers rebuild
	b.Publish(Frame{Seq: 1, Data: []byte("test")})

	// Cache should be clean now
	if b.cacheDirty.Load() {
		t.Error("Expected cache to be clean after first Publish")
	}

	// Cache should be populated with correct order
	if len(b.sortedCache) != 3 {
		t.Errorf("Expected 3 entries in cache, got %d", len(b.sortedCache))
	}

	// Verify order: Critical, Normal, BestEffort
	if b.sortedCache[0].entry.priority != PriorityCritical {
		t.Errorf("Expected first entry to be Critical, got %d", b.sortedCache[0].entry.priority)
	}
	if b.sortedCache[1].entry.priority != PriorityNormal {
		t.Errorf("Expected second entry to be Normal, got %d", b.sortedCache[1].entry.priority)
	}
	if b.sortedCache[2].entry.priority != PriorityBestEffort {
		t.Errorf("Expected third entry to be BestEffort, got %d", b.sortedCache[2].entry.priority)
	}
}

// TestEventualConsistency verifies streaming semantics (eventual consistency).
func TestEventualConsistency(t *testing.T) {
	b := New()
	defer b.Close()

	// Subscribe initial worker
	ch1 := make(chan Frame, 10)
	b.Subscribe("worker-1", ch1)

	// First Publish - only worker-1 receives
	b.Publish(Frame{Seq: 1, Data: []byte("frame1")})

	stats := b.Stats()
	if stats.Subscribers["worker-1"].Sent != 1 {
		t.Errorf("Expected worker-1 to receive 1 frame, got %d", stats.Subscribers["worker-1"].Sent)
	}

	// Subscribe second worker AFTER first Publish
	ch2 := make(chan Frame, 10)
	b.Subscribe("worker-2", ch2)

	// Second Publish - both workers receive (eventual consistency)
	b.Publish(Frame{Seq: 2, Data: []byte("frame2")})

	stats = b.Stats()
	if stats.Subscribers["worker-1"].Sent != 2 {
		t.Errorf("Expected worker-1 to receive 2 frames, got %d", stats.Subscribers["worker-1"].Sent)
	}
	if stats.Subscribers["worker-2"].Sent != 1 {
		t.Errorf("Expected worker-2 to receive 1 frame (not frame1), got %d", stats.Subscribers["worker-2"].Sent)
	}

	// worker-2 did NOT receive frame1 (eventual consistency - not retroactive)
	if len(ch2) != 1 {
		t.Errorf("Expected worker-2 channel to have 1 frame, got %d", len(ch2))
	}
}

// TestBatchSubscribeOptimization verifies that multiple Subscribes result in 1 rebuild.
func TestBatchSubscribeOptimization(t *testing.T) {
	b := New().(*bus)
	defer b.Close()

	// Subscribe 10 workers rapidly
	for i := 0; i < 10; i++ {
		ch := make(chan Frame, 10)
		b.Subscribe(fmt.Sprintf("worker-%d", i), ch)
	}

	// All subscribes marked cache dirty
	if !b.cacheDirty.Load() {
		t.Error("Expected cache to be dirty after subscribes")
	}

	// Cache still empty (not rebuilt yet - lazy!)
	if len(b.sortedCache) != 0 {
		t.Error("Expected cache to be empty before Publish (lazy rebuild)")
	}

	// First Publish triggers ONE rebuild for all 10 subscribes
	b.Publish(Frame{Seq: 1, Data: []byte("test")})

	// Cache now populated
	if len(b.sortedCache) != 10 {
		t.Errorf("Expected 10 entries in cache, got %d", len(b.sortedCache))
	}

	// Cache clean
	if b.cacheDirty.Load() {
		t.Error("Expected cache to be clean after Publish")
	}
}

// TestUnsubscribeInvalidatesCache verifies Unsubscribe marks cache dirty.
func TestUnsubscribeInvalidatesCache(t *testing.T) {
	b := New().(*bus)
	defer b.Close()

	ch1 := make(chan Frame, 10)
	ch2 := make(chan Frame, 10)
	b.Subscribe("worker-1", ch1)
	b.Subscribe("worker-2", ch2)

	// Trigger rebuild
	b.Publish(Frame{Seq: 1, Data: []byte("test")})

	// Cache clean
	if b.cacheDirty.Load() {
		t.Error("Expected cache to be clean after Publish")
	}

	// Unsubscribe marks dirty
	b.Unsubscribe("worker-1")

	if !b.cacheDirty.Load() {
		t.Error("Expected cache to be dirty after Unsubscribe")
	}

	// Next Publish rebuilds cache
	b.Publish(Frame{Seq: 2, Data: []byte("test")})

	// Cache clean again
	if b.cacheDirty.Load() {
		t.Error("Expected cache to be clean after Publish following Unsubscribe")
	}

	// Cache has correct size
	if len(b.sortedCache) != 1 {
		t.Errorf("Expected 1 entry in cache after Unsubscribe, got %d", len(b.sortedCache))
	}
}

// TestSmartCheckAllSamePriority verifies no sorting when all same priority.
func TestSmartCheckAllSamePriority(t *testing.T) {
	b := New().(*bus)
	defer b.Close()

	// Subscribe 5 workers, all Normal priority
	for i := 0; i < 5; i++ {
		ch := make(chan Frame, 10)
		b.Subscribe(fmt.Sprintf("worker-%d", i), ch)
	}

	// Check needsSorting before rebuild
	needsSort := b.needsSorting()
	if needsSort {
		t.Error("Expected needsSorting=false for all same priority (Normal)")
	}

	// Trigger rebuild
	b.Publish(Frame{Seq: 1, Data: []byte("test")})

	// Verify cache populated (even without sorting)
	if len(b.sortedCache) != 5 {
		t.Errorf("Expected cache to have 5 entries, got %d", len(b.sortedCache))
	}

	// All should have Normal priority
	for _, sub := range b.sortedCache {
		if sub.entry.priority != PriorityNormal {
			t.Errorf("Expected all entries to be Normal, got %d", sub.entry.priority)
		}
	}
}

// TestSmartCheckMixedPriorities verifies sorting when priorities are mixed.
func TestSmartCheckMixedPriorities(t *testing.T) {
	b := New().(*bus)
	defer b.Close()

	// Subscribe 3 workers with mixed priorities
	ch1 := make(chan Frame, 10)
	ch2 := make(chan Frame, 10)
	ch3 := make(chan Frame, 10)

	b.SubscribeWithPriority("critical", ch1, PriorityCritical)
	b.SubscribeWithPriority("normal", ch2, PriorityNormal)
	b.SubscribeWithPriority("best-effort", ch3, PriorityBestEffort)

	// Check needsSorting before rebuild
	needsSort := b.needsSorting()
	if !needsSort {
		t.Error("Expected needsSorting=true for mixed priorities")
	}

	// Trigger rebuild
	b.Publish(Frame{Seq: 1, Data: []byte("test")})

	// Verify cache populated and sorted
	if len(b.sortedCache) != 3 {
		t.Errorf("Expected cache to have 3 entries, got %d", len(b.sortedCache))
	}

	// Verify order: Critical, Normal, BestEffort
	if b.sortedCache[0].entry.priority != PriorityCritical {
		t.Errorf("Expected first entry to be Critical, got %d", b.sortedCache[0].entry.priority)
	}
	if b.sortedCache[1].entry.priority != PriorityNormal {
		t.Errorf("Expected second entry to be Normal, got %d", b.sortedCache[1].entry.priority)
	}
	if b.sortedCache[2].entry.priority != PriorityBestEffort {
		t.Errorf("Expected third entry to be BestEffort, got %d", b.sortedCache[2].entry.priority)
	}
}

// TestSmartCheckSingleSubscriber verifies no sorting for 1 subscriber.
func TestSmartCheckSingleSubscriber(t *testing.T) {
	b := New().(*bus)
	defer b.Close()

	ch := make(chan Frame, 10)
	b.SubscribeWithPriority("worker-1", ch, PriorityCritical)

	// Check needsSorting
	needsSort := b.needsSorting()
	if needsSort {
		t.Error("Expected needsSorting=false for single subscriber")
	}

	// Trigger rebuild
	b.Publish(Frame{Seq: 1, Data: []byte("test")})

	// Verify cache has 1 entry
	if len(b.sortedCache) != 1 {
		t.Errorf("Expected cache to have 1 entry, got %d", len(b.sortedCache))
	}
}

// TestSmartCheckOrionScenarios verifies optimization in real Orion use cases.
func TestSmartCheckOrionScenarios(t *testing.T) {
	// Scenario 1: Sleep monitoring (all Normal)
	t.Run("SleepScene_AllNormal", func(t *testing.T) {
		b := New().(*bus)
		defer b.Close()

		// SleepExpert, PostureExpert, CaregiverExpert - all Normal
		b.Subscribe("sleep-expert", make(chan Frame, 10))
		b.Subscribe("posture-expert", make(chan Frame, 10))
		b.Subscribe("caregiver-expert", make(chan Frame, 10))

		if b.needsSorting() {
			t.Error("Sleep scene should not need sorting (all Normal)")
		}
	})

	// Scenario 2: Alert state (all Critical)
	t.Run("AlertScene_AllCritical", func(t *testing.T) {
		b := New().(*bus)
		defer b.Close()

		// EdgeExpert, ExitExpert - both Critical during alert
		b.SubscribeWithPriority("edge-expert", make(chan Frame, 10), PriorityCritical)
		b.SubscribeWithPriority("exit-expert", make(chan Frame, 10), PriorityCritical)

		if b.needsSorting() {
			t.Error("Alert scene should not need sorting (all Critical)")
		}
	})

	// Scenario 3: Normal state (mixed priorities)
	t.Run("NormalScene_MixedPriorities", func(t *testing.T) {
		b := New().(*bus)
		defer b.Close()

		// EdgeExpert (High), PostureExpert (Normal), CaregiverExpert (BestEffort)
		b.SubscribeWithPriority("edge-expert", make(chan Frame, 10), PriorityHigh)
		b.SubscribeWithPriority("posture-expert", make(chan Frame, 10), PriorityNormal)
		b.SubscribeWithPriority("caregiver-expert", make(chan Frame, 10), PriorityBestEffort)

		if !b.needsSorting() {
			t.Error("Normal scene should need sorting (mixed priorities)")
		}
	})
}

// BenchmarkLazyRebuildSingleSubscriber measures hot path with 1 subscriber (70-80% use case).
func BenchmarkLazyRebuildSingleSubscriber(b *testing.B) {
	bus := New()
	defer bus.Close()

	// Single subscriber (most common case in production)
	ch := make(chan Frame, 100)
	bus.Subscribe("worker-1", ch)

	// Consumer draining
	go func() {
		for range ch {
		}
	}()

	frame := Frame{Seq: 1, Data: make([]byte, 1024)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Publish(frame)
	}
}

// BenchmarkLazyRebuildTenSubscribers measures hot path with 10 subscribers (after rebuild).
func BenchmarkLazyRebuildTenSubscribers(b *testing.B) {
	bus := New()
	defer bus.Close()

	// 10 subscribers with mixed priorities
	for i := 0; i < 10; i++ {
		ch := make(chan Frame, 100)
		priority := SubscriberPriority(i % 4)
		bus.SubscribeWithPriority(fmt.Sprintf("worker-%d", i), ch, priority)

		// Consumer draining
		go func(ch chan Frame) {
			for range ch {
			}
		}(ch)
	}

	// Trigger initial rebuild (not measured)
	bus.Publish(Frame{Seq: 0, Data: []byte("warmup")})

	frame := Frame{Seq: 1, Data: make([]byte, 1024)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Publish(frame) // Hot path - cache already rebuilt
	}
}

// BenchmarkRebuildOperation measures cost of rebuild operation itself.
func BenchmarkRebuildOperation(b *testing.B) {
	// Create bus with 10 subscribers
	bus := New().(*bus)
	defer bus.Close()

	for i := 0; i < 10; i++ {
		ch := make(chan Frame, 10)
		priority := SubscriberPriority(i % 4)
		bus.SubscribeWithPriority(fmt.Sprintf("worker-%d", i), ch, priority)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Measure only the sorting operation
		bus.sortSubscribersByPriority()
	}
}

// BenchmarkBatchSubscribeLatency measures latency of multiple Subscribes (cold path).
func BenchmarkBatchSubscribeLatency(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		bus := New()
		b.StartTimer()

		// Subscribe 10 workers (measure Subscribe latency)
		for j := 0; j < 10; j++ {
			ch := make(chan Frame, 10)
			bus.Subscribe(fmt.Sprintf("worker-%d", j), ch)
		}

		b.StopTimer()
		bus.Close()
	}
}

// BenchmarkSmartCheckAllSamePriority measures rebuild with all same priority (no sorting).
func BenchmarkSmartCheckAllSamePriority(b *testing.B) {
	bus := New().(*bus)
	defer bus.Close()

	// 10 workers, all Normal priority
	for i := 0; i < 10; i++ {
		ch := make(chan Frame, 10)
		bus.Subscribe(fmt.Sprintf("worker-%d", i), ch)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Measure rebuild cost (smart check detects no sorting needed)
		bus.cacheDirty.Store(true) // Force rebuild
		bus.sortedCache = nil

		if bus.needsSorting() {
			bus.sortedCache = bus.sortSubscribersByPriority()
		} else {
			bus.sortedCache = bus.subscribersToSlice()
		}
	}
}

// BenchmarkSmartCheckMixedPriorities measures rebuild with mixed priorities (with sorting).
func BenchmarkSmartCheckMixedPriorities(b *testing.B) {
	bus := New().(*bus)
	defer bus.Close()

	// 10 workers, mixed priorities
	for i := 0; i < 10; i++ {
		ch := make(chan Frame, 10)
		priority := SubscriberPriority(i % 4) // Mix of 4 levels
		bus.SubscribeWithPriority(fmt.Sprintf("worker-%d", i), ch, priority)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Measure rebuild cost (smart check detects sorting needed)
		bus.cacheDirty.Store(true)
		bus.sortedCache = nil

		if bus.needsSorting() {
			bus.sortedCache = bus.sortSubscribersByPriority()
		} else {
			bus.sortedCache = bus.subscribersToSlice()
		}
	}
}

// BenchmarkPublishAllSamePriority measures Publish() hot path with all same priority.
func BenchmarkPublishAllSamePriority(b *testing.B) {
	bus := New()
	defer bus.Close()

	// 5 workers, all Normal (typical sleep scene in Orion)
	for i := 0; i < 5; i++ {
		ch := make(chan Frame, 100)
		bus.Subscribe(fmt.Sprintf("worker-%d", i), ch)

		// Consumer draining
		go func(ch chan Frame) {
			for range ch {
			}
		}(ch)
	}

	// Trigger initial rebuild
	bus.Publish(Frame{Seq: 0, Data: []byte("warmup")})

	frame := Frame{Seq: 1, Data: make([]byte, 1024)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Publish(frame)
	}
}

// BenchmarkPublishMixedPriorities measures Publish() hot path with mixed priorities.
func BenchmarkPublishMixedPriorities(b *testing.B) {
	bus := New()
	defer bus.Close()

	// 5 workers, mixed priorities (typical normal scene in Orion)
	priorities := []SubscriberPriority{
		PriorityHigh,      // EdgeExpert
		PriorityNormal,    // PostureExpert
		PriorityNormal,    // CaregiverExpert
		PriorityBestEffort, // Research worker
		PriorityBestEffort, // Telemetry
	}

	for i, priority := range priorities {
		ch := make(chan Frame, 100)
		bus.SubscribeWithPriority(fmt.Sprintf("worker-%d", i), ch, priority)

		// Consumer draining
		go func(ch chan Frame) {
			for range ch {
			}
		}(ch)
	}

	// Trigger initial rebuild
	bus.Publish(Frame{Seq: 0, Data: []byte("warmup")})

	frame := Frame{Seq: 1, Data: make([]byte, 1024)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Publish(frame)
	}
}
