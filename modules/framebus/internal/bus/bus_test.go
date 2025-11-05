package bus

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestNewBus(t *testing.T) {
	bus := New()
	if bus == nil {
		t.Fatal("New() returned nil")
	}
}

func TestSubscribeDropNew(t *testing.T) {
	b := New()
	defer b.Close()

	ch := make(chan Frame, 1)
	err := b.Subscribe("worker-1", ch)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Duplicate subscription should fail
	err = b.Subscribe("worker-1", ch)
	if err != ErrSubscriberExists {
		t.Errorf("Expected ErrSubscriberExists, got %v", err)
	}
}

func TestSubscribeDropOld(t *testing.T) {
	b := New()
	defer b.Close()

	receiver, err := b.SubscribeDropOld("worker-2")
	if err != nil {
		t.Fatalf("SubscribeDropOld failed: %v", err)
	}
	defer receiver.Close()

	if receiver == nil {
		t.Fatal("SubscribeDropOld returned nil receiver")
	}
}

func TestPublishDropNew(t *testing.T) {
	b := New()
	defer b.Close()

	ch := make(chan Frame, 2)
	b.Subscribe("worker-1", ch)

	frame1 := Frame{Sequence: 1, Data: []byte("frame1")}
	frame2 := Frame{Sequence: 2, Data: []byte("frame2")}

	b.Publish(frame1)
	b.Publish(frame2)

	received1 := <-ch
	if received1.Sequence != 1 {
		t.Errorf("Expected sequence 1, got %d", received1.Sequence)
	}

	received2 := <-ch
	if received2.Sequence != 2 {
		t.Errorf("Expected sequence 2, got %d", received2.Sequence)
	}
}

func TestPublishDropOld(t *testing.T) {
	b := New()
	defer b.Close()

	receiver, _ := b.SubscribeDropOld("worker-2")
	defer receiver.Close()

	frame1 := Frame{Sequence: 1, Data: []byte("frame1")}
	frame2 := Frame{Sequence: 2, Data: []byte("frame2")}

	b.Publish(frame1)
	b.Publish(frame2)

	// Should get latest frame (frame2)
	received, ok := receiver.TryReceive()
	if !ok {
		t.Fatal("TryReceive returned false")
	}
	if received.Sequence != 2 {
		t.Errorf("Expected sequence 2, got %d", received.Sequence)
	}
}

func TestDropNewBackpressure(t *testing.T) {
	b := New()
	defer b.Close()

	ch := make(chan Frame, 1) // Buffer of 1
	b.Subscribe("worker-1", ch)

	// Fill buffer
	b.Publish(Frame{Sequence: 1})
	// This should be dropped
	b.Publish(Frame{Sequence: 2})

	stats, _ := b.Stats("worker-1")
	if stats.Sent != 1 {
		t.Errorf("Expected 1 sent, got %d", stats.Sent)
	}
	if stats.Dropped != 1 {
		t.Errorf("Expected 1 dropped, got %d", stats.Dropped)
	}
}

func TestDropOldReplacement(t *testing.T) {
	b := New()
	defer b.Close()

	receiver, _ := b.SubscribeDropOld("worker-2")
	defer receiver.Close()

	// Publish multiple frames rapidly
	for i := uint64(1); i <= 10; i++ {
		b.Publish(Frame{Sequence: i})
	}

	// Should get the latest (10)
	received, ok := receiver.TryReceive()
	if !ok {
		t.Fatal("TryReceive returned false")
	}
	if received.Sequence != 10 {
		t.Errorf("Expected sequence 10, got %d", received.Sequence)
	}

	// No drops should be recorded for DropOld
	stats, _ := b.Stats("worker-2")
	if stats.Dropped != 0 {
		t.Errorf("Expected 0 dropped for DropOld, got %d", stats.Dropped)
	}
}

func TestReceiveBlocking(t *testing.T) {
	b := New()
	defer b.Close()

	receiver, _ := b.SubscribeDropOld("worker-3")
	defer receiver.Close()

	done := make(chan bool)
	var received Frame

	// Goroutine blocks waiting for frame
	go func() {
		received = receiver.Receive()
		done <- true
	}()

	// Give goroutine time to start waiting
	time.Sleep(10 * time.Millisecond)

	// Publish frame
	b.Publish(Frame{Sequence: 42, Data: []byte("test")})

	// Wait for receive
	select {
	case <-done:
		if received.Sequence != 42 {
			t.Errorf("Expected sequence 42, got %d", received.Sequence)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Receive() did not unblock")
	}
}

func TestUnsubscribe(t *testing.T) {
	b := New()
	defer b.Close()

	ch := make(chan Frame, 1)
	b.Subscribe("worker-1", ch)

	err := b.Unsubscribe("worker-1")
	if err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}

	// Second unsubscribe should fail
	err = b.Unsubscribe("worker-1")
	if err != ErrSubscriberNotFound {
		t.Errorf("Expected ErrSubscriberNotFound, got %v", err)
	}
}

func TestStats(t *testing.T) {
	b := New()
	defer b.Close()

	ch := make(chan Frame, 10)
	b.Subscribe("worker-1", ch)

	for i := 0; i < 5; i++ {
		b.Publish(Frame{Sequence: uint64(i)})
	}

	stats, err := b.Stats("worker-1")
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}

	if stats.Sent != 5 {
		t.Errorf("Expected 5 sent, got %d", stats.Sent)
	}
	if stats.Dropped != 0 {
		t.Errorf("Expected 0 dropped, got %d", stats.Dropped)
	}
}

func TestCloseBus(t *testing.T) {
	b := New()
	receiver, _ := b.SubscribeDropOld("worker-1")

	b.Close()

	// Operations after close should fail
	err := b.Subscribe("worker-2", make(chan Frame))
	if err != ErrBusClosed {
		t.Errorf("Expected ErrBusClosed, got %v", err)
	}

	// Receiver should be closed
	frame := receiver.Receive()
	if frame.Sequence != 0 {
		t.Error("Expected zero frame after bus close")
	}
}

func TestConcurrentPublish(t *testing.T) {
	b := New()
	defer b.Close()

	receiver, _ := b.SubscribeDropOld("worker-1")
	defer receiver.Close()

	const goroutines = 10
	const framesPerGoroutine = 100

	var published atomic.Uint64

	for i := 0; i < goroutines; i++ {
		go func() {
			for j := 0; j < framesPerGoroutine; j++ {
				seq := published.Add(1)
				b.Publish(Frame{Sequence: seq})
			}
		}()
	}

	time.Sleep(100 * time.Millisecond)

	// Should have latest frame
	_, ok := receiver.TryReceive()
	if !ok {
		t.Fatal("Expected to receive frame")
	}
}
