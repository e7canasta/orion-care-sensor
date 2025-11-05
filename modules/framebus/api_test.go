package framebus_test

import (
	"testing"

	"github.com/e7canasta/orion-care-sensor/modules/framebus"
)

// TestPublicAPIContract validates the public API surface remains stable
// These tests ensure we don't accidentally break the public contract

func TestPublicAPI_New(t *testing.T) {
	bus := framebus.New()
	if bus == nil {
		t.Fatal("New() should return non-nil Bus")
	}
}

func TestPublicAPI_SubscribeChannel(t *testing.T) {
	bus := framebus.New()
	defer bus.Close()

	ch := make(chan framebus.Frame, 1)
	
	// Should accept channel subscription
	err := bus.Subscribe("test", ch)
	if err != nil {
		t.Fatalf("Subscribe() should succeed: %v", err)
	}

	// Should reject duplicate
	err = bus.Subscribe("test", ch)
	if err != framebus.ErrSubscriberExists {
		t.Errorf("Subscribe() duplicate should return ErrSubscriberExists, got %v", err)
	}

	// Should reject nil channel
	err = bus.Subscribe("test2", nil)
	if err != framebus.ErrNilChannel {
		t.Errorf("Subscribe(nil) should return ErrNilChannel, got %v", err)
	}
}

func TestPublicAPI_SubscribeDropOld(t *testing.T) {
	bus := framebus.New()
	defer bus.Close()

	receiver, err := bus.SubscribeDropOld("test")
	if err != nil {
		t.Fatalf("SubscribeDropOld() should succeed: %v", err)
	}
	if receiver == nil {
		t.Fatal("SubscribeDropOld() should return non-nil receiver")
	}

	// Verify interface compliance
	var _ framebus.FrameReceiver = receiver
}

func TestPublicAPI_Publish(t *testing.T) {
	bus := framebus.New()
	defer bus.Close()

	ch := make(chan framebus.Frame, 1)
	bus.Subscribe("test", ch)

	frame := framebus.Frame{
		Data:      []byte("test"),
		Width:     640,
		Height:    480,
		Sequence:  1,
		Timestamp: 1234567890,
		Meta:      map[string]interface{}{"key": "value"},
	}

	// Should not panic
	bus.Publish(frame)

	// Should receive frame
	received := <-ch
	if received.Sequence != 1 {
		t.Errorf("Expected sequence 1, got %d", received.Sequence)
	}
}

func TestPublicAPI_Stats(t *testing.T) {
	bus := framebus.New()
	defer bus.Close()

	ch := make(chan framebus.Frame, 1)
	bus.Subscribe("test", ch)

	stats, err := bus.Stats("test")
	if err != nil {
		t.Fatalf("Stats() should succeed: %v", err)
	}
	if stats == nil {
		t.Fatal("Stats() should return non-nil stats")
	}

	// Stats should have expected fields
	_ = stats.Sent
	_ = stats.Dropped
}

func TestPublicAPI_Unsubscribe(t *testing.T) {
	bus := framebus.New()
	defer bus.Close()

	ch := make(chan framebus.Frame, 1)
	bus.Subscribe("test", ch)

	err := bus.Unsubscribe("test")
	if err != nil {
		t.Fatalf("Unsubscribe() should succeed: %v", err)
	}

	// Second unsubscribe should fail
	err = bus.Unsubscribe("test")
	if err != framebus.ErrSubscriberNotFound {
		t.Errorf("Unsubscribe() non-existent should return ErrSubscriberNotFound, got %v", err)
	}
}

func TestPublicAPI_Close(t *testing.T) {
	bus := framebus.New()
	bus.Close()

	// Should reject operations after close
	err := bus.Subscribe("test", make(chan framebus.Frame))
	if err != framebus.ErrBusClosed {
		t.Errorf("Operations after Close() should return ErrBusClosed, got %v", err)
	}
}

func TestPublicAPI_FrameReceiverInterface(t *testing.T) {
	bus := framebus.New()
	defer bus.Close()

	receiver, _ := bus.SubscribeDropOld("test")
	defer receiver.Close()

	// Publish frame
	bus.Publish(framebus.Frame{Sequence: 42})

	// TryReceive should work
	frame, ok := receiver.TryReceive()
	if !ok {
		t.Fatal("TryReceive() should return true")
	}
	if frame.Sequence != 42 {
		t.Errorf("TryReceive() should return frame 42, got %d", frame.Sequence)
	}

	// Receive should work
	bus.Publish(framebus.Frame{Sequence: 43})
	frame = receiver.Receive()
	if frame.Sequence != 43 {
		t.Errorf("Receive() should return frame 43, got %d", frame.Sequence)
	}

	// Close should work
	receiver.Close()
}

func TestPublicAPI_Errors(t *testing.T) {
	// All public errors should be defined
	errors := []error{
		framebus.ErrBusClosed,
		framebus.ErrSubscriberExists,
		framebus.ErrSubscriberNotFound,
		framebus.ErrNilChannel,
		framebus.ErrReceiverClosed,
	}

	for _, err := range errors {
		if err == nil {
			t.Error("Public error should not be nil")
		}
		if err.Error() == "" {
			t.Error("Public error should have message")
		}
	}
}

func TestPublicAPI_DropPolicy(t *testing.T) {
	// Drop policies should be defined
	_ = framebus.DropNew
	_ = framebus.DropOld

	// Should be distinct values
	if framebus.DropNew == framebus.DropOld {
		t.Error("DropNew and DropOld should be different values")
	}
}
