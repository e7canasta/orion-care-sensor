package framebus

import "github.com/e7canasta/orion-care-sensor/modules/framebus/internal/bus"

// Public API - Re-export internal types as stable contract

// DropPolicy defines how the bus handles frames when subscriber cannot keep up
type DropPolicy = bus.DropPolicy

const (
	// DropNew drops incoming frames if subscriber's buffer is full (backpressure)
	DropNew = bus.DropNew
	// DropOld always accepts new frames, replacing old ones (latest-only)
	DropOld = bus.DropOld
)

// Frame represents a video frame with metadata
type Frame = bus.Frame

// FrameReceiver provides blocking/non-blocking frame access for DropOld policy
type FrameReceiver = bus.FrameReceiver

// SubscriberStats tracks frame distribution metrics
type SubscriberStats = bus.SubscriberStats

// Bus distributes frames to multiple subscribers with configurable drop policies
type Bus = bus.Bus

// Public API errors - Re-export internal errors as stable contract
var (
	ErrBusClosed          = bus.ErrBusClosed
	ErrSubscriberExists   = bus.ErrSubscriberExists
	ErrSubscriberNotFound = bus.ErrSubscriberNotFound
	ErrNilChannel         = bus.ErrNilChannel
	ErrReceiverClosed     = bus.ErrReceiverClosed
)

