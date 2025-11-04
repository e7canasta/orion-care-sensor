package framebus

import (
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
